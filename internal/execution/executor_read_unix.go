//go:build unix

package execution

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

const (
	defaultReadOutputLimit = int64(12 << 10)
	maxReadOutputLimit     = int64(12 << 10)
	maxReadFileBytes       = int64(8 << 20)
	defaultGrepMatchLimit  = 100
	maxGrepMatchLimit      = 1000
	maxGrepFiles           = 2000
	maxGrepBytes           = int64(16 << 20)
)

func (e *SandboxExecutor) Read(
	ctx context.Context,
	request ReadRequest,
) (ReadResult, error) {
	started := time.Now()
	content, err := readSecureWorkspaceText(ctx, request.Worktree, request.RelativePath)
	if err != nil {
		return ReadResult{}, err
	}
	defer zeroExecutionBytes(content)
	limit, err := boundedReadOutputLimit(request.OutputLimit)
	if err != nil {
		return ReadResult{}, err
	}
	output, truncated := boundedUTF8Prefix(content, limit)
	if len(output) == 0 {
		output, truncated = boundedUTF8Prefix([]byte("File is empty.\n"), limit)
	}
	return ReadResult{
		Content:       output,
		ContentDigest: digestBytes(output),
		OutputDigest:  digestBytes(content),
		Truncated:     truncated,
		DurationMS:    time.Since(started).Milliseconds(),
	}, nil
}

func (e *SandboxExecutor) Grep(
	ctx context.Context,
	request GrepRequest,
) (GrepResult, error) {
	started := time.Now()
	pattern, err := validateGrepPattern(request.Pattern)
	if err != nil {
		return GrepResult{}, ErrInvalidExecutionInput
	}
	limit, err := boundedReadOutputLimit(request.OutputLimit)
	if err != nil {
		return GrepResult{}, err
	}
	matchLimit := request.MatchLimit
	if matchLimit <= 0 {
		matchLimit = defaultGrepMatchLimit
	}
	if matchLimit > maxGrepMatchLimit {
		return GrepResult{}, ErrExecutionLimit
	}
	var output []byte
	truncated := false
	matches := 0
	root, err := secureWorktreeRoot(request.Worktree)
	if err != nil {
		return GrepResult{}, err
	}
	relative, err := secureGrepRelativePath(request.RelativePath)
	if err != nil {
		return GrepResult{}, err
	}
	target := filepath.Join(root, relative)
	if relative != "." && !pathWithin(root, target) {
		return GrepResult{}, ErrExecutionPathOutside
	}
	validationDirectory := filepath.Dir(target)
	if relative == "." {
		validationDirectory = root
	}
	if err := ensureNoSymlinkComponents(root, validationDirectory); err != nil {
		return GrepResult{}, err
	}
	info, err := os.Lstat(target)
	if err != nil {
		return GrepResult{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return GrepResult{}, ErrExecutionPathOutside
	}
	files := []string{relative}
	if info.IsDir() {
		files = files[:0]
		err = filepath.WalkDir(target, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return ErrExecutionPathOutside
			}
			if entry.IsDir() {
				return nil
			}
			if !entry.Type().IsRegular() || len(files) >= maxGrepFiles {
				return ErrExecutionLimit
			}
			fileRelative, relErr := filepath.Rel(root, path)
			if relErr != nil || fileRelative == "." || !pathWithin(root, path) {
				return ErrExecutionPathOutside
			}
			files = append(files, fileRelative)
			return nil
		})
		if err != nil {
			return GrepResult{}, err
		}
	} else if !info.Mode().IsRegular() {
		return GrepResult{}, ErrExecutionPathOutside
	}
	scannedBytes := int64(0)
	for _, file := range files {
		content, readErr := readSecureWorkspaceText(ctx, root, file)
		if readErr != nil {
			zeroExecutionBytes(output)
			return GrepResult{}, readErr
		}
		scannedBytes += int64(len(content))
		if scannedBytes > maxGrepBytes {
			zeroExecutionBytes(content)
			zeroExecutionBytes(output)
			return GrepResult{}, ErrExecutionLimit
		}
		lines := bytes.Split(content, []byte{'\n'})
		for index, line := range lines {
			if err := ctx.Err(); err != nil {
				zeroExecutionBytes(content)
				zeroExecutionBytes(output)
				return GrepResult{}, err
			}
			if !pattern.Match(line) {
				continue
			}
			matches++
			prefix := ""
			if len(files) > 1 || info.IsDir() {
				prefix = file + ":"
			}
			entry := []byte(prefix + strconv.Itoa(index+1) + ":")
			entry = append(entry, line...)
			entry = append(entry, '\n')
			if matches > matchLimit || int64(len(output)+len(entry)) > limit {
				zeroExecutionBytes(entry)
				truncated = true
				break
			}
			output = append(output, entry...)
			zeroExecutionBytes(entry)
		}
		zeroExecutionBytes(content)
		if truncated {
			break
		}
	}
	if len(output) == 0 && !truncated {
		output, truncated = boundedUTF8Prefix([]byte("No matches.\n"), limit)
	}
	return GrepResult{
		Content:       output,
		ContentDigest: digestBytes(output),
		OutputDigest:  digestBytes(output),
		Truncated:     truncated,
		DurationMS:    time.Since(started).Milliseconds(),
	}, nil
}

func boundedReadOutputLimit(value int64) (int64, error) {
	if value <= 0 {
		return defaultReadOutputLimit, nil
	}
	if value > maxReadOutputLimit {
		return 0, ErrExecutionLimit
	}
	return value, nil
}

func boundedUTF8Prefix(content []byte, limit int64) ([]byte, bool) {
	if int64(len(content)) <= limit {
		return bytes.Clone(content), false
	}
	end := int(limit)
	for end > 0 && !utf8.Valid(content[:end]) {
		end--
	}
	return bytes.Clone(content[:end]), true
}

func readSecureWorkspaceText(
	ctx context.Context,
	worktree string,
	relativePath string,
) ([]byte, error) {
	if ctx == nil || ctx.Err() != nil {
		return nil, context.Canceled
	}
	root, err := secureWorktreeRoot(worktree)
	if err != nil {
		return nil, err
	}
	relative, err := secureRelativePath(relativePath)
	if err != nil {
		return nil, err
	}
	components := strings.Split(filepath.ToSlash(relative), "/")
	if len(components) == 0 {
		return nil, ErrExecutionPathOutside
	}
	rootFD, err := unix.Open(
		root,
		unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC,
		0,
	)
	if err != nil {
		return nil, ErrExecutionPathOutside
	}
	currentFD := rootFD
	defer unix.Close(rootFD)
	defer func() {
		if currentFD != rootFD {
			_ = unix.Close(currentFD)
		}
	}()
	for _, component := range components[:len(components)-1] {
		if component == "" || component == "." || component == ".." {
			return nil, ErrExecutionPathOutside
		}
		nextFD, openErr := unix.Openat(
			currentFD,
			component,
			unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC,
			0,
		)
		if openErr != nil {
			return nil, ErrExecutionPathOutside
		}
		if currentFD != rootFD {
			_ = unix.Close(currentFD)
		}
		currentFD = nextFD
	}
	name := components[len(components)-1]
	if name == "" || name == "." || name == ".." {
		return nil, ErrExecutionPathOutside
	}
	var before unix.Stat_t
	if err := unix.Fstatat(currentFD, name, &before, unix.AT_SYMLINK_NOFOLLOW); err != nil ||
		uint32(before.Mode)&unix.S_IFMT != unix.S_IFREG || before.Nlink != 1 ||
		before.Size < 0 || before.Size > maxReadFileBytes {
		return nil, ErrExecutionPathOutside
	}
	fileFD, err := unix.Openat(
		currentFD,
		name,
		unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC,
		0,
	)
	if err != nil {
		return nil, ErrExecutionPathOutside
	}
	file := os.NewFile(uintptr(fileFD), relative)
	if file == nil {
		_ = unix.Close(fileFD)
		return nil, ErrExecutionContent
	}
	defer file.Close()
	var opened unix.Stat_t
	if err := unix.Fstat(fileFD, &opened); err != nil ||
		!sameExecutionFileIdentity(&before, &opened) {
		return nil, ErrExecutionPathOutside
	}
	content, err := io.ReadAll(io.LimitReader(file, maxReadFileBytes+1))
	if err != nil || int64(len(content)) > maxReadFileBytes {
		zeroExecutionBytes(content)
		return nil, ErrExecutionLimit
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		zeroExecutionBytes(content)
		return nil, ErrExecutionContent
	}
	verification, err := io.ReadAll(io.LimitReader(file, maxReadFileBytes+1))
	if err != nil || !bytes.Equal(content, verification) {
		zeroExecutionBytes(content)
		zeroExecutionBytes(verification)
		return nil, ErrExecutionContent
	}
	zeroExecutionBytes(verification)
	var after unix.Stat_t
	var pathAfter unix.Stat_t
	if err := unix.Fstat(fileFD, &after); err != nil ||
		unix.Fstatat(currentFD, name, &pathAfter, unix.AT_SYMLINK_NOFOLLOW) != nil ||
		!sameExecutionFileIdentity(&opened, &after) ||
		!sameExecutionFileIdentity(&after, &pathAfter) ||
		int64(len(content)) != after.Size {
		zeroExecutionBytes(content)
		return nil, ErrExecutionPathOutside
	}
	if !utf8.Valid(content) || executionTextHasUnsafeControls(content) {
		zeroExecutionBytes(content)
		return nil, ErrExecutionContent
	}
	return content, nil
}

func sameExecutionFileIdentity(left, right *unix.Stat_t) bool {
	return left != nil && right != nil &&
		left.Dev == right.Dev && left.Ino == right.Ino &&
		left.Mode == right.Mode && left.Nlink == right.Nlink &&
		left.Size == right.Size && uint32(right.Mode)&unix.S_IFMT == unix.S_IFREG &&
		right.Nlink == 1
}

func zeroExecutionBytes(content []byte) {
	for index := range content {
		content[index] = 0
	}
}
