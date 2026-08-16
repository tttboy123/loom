package harnessadapter

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

type systemHarnessCommandRunner struct{}

type harnessExecutableIdentity struct {
	path          string
	directoryPath string
	file          os.FileInfo
	directory     os.FileInfo
}

type boundedHarnessBuffer struct {
	buffer   bytes.Buffer
	maximum  int
	overflow bool
}

func NewSystemHarnessCommandRunner() HarnessCommandRunner {
	return systemHarnessCommandRunner{}
}

func ResolveHarnessExecutable(path string) (string, error) {
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) || cleaned != path {
		return "", ErrHarnessProcessUnavailable
	}
	info, err := os.Lstat(cleaned)
	if err != nil {
		return "", ErrHarnessProcessUnavailable
	}
	if info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
		return cleaned, nil
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", ErrHarnessProcessUnavailable
	}
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		return "", ErrHarnessProcessUnavailable
	}
	resolved = filepath.Clean(resolved)
	resolvedInfo, err := os.Lstat(resolved)
	if err != nil || !filepath.IsAbs(resolved) || !resolvedInfo.Mode().IsRegular() ||
		resolvedInfo.Mode()&0o111 == 0 {
		return "", ErrHarnessProcessUnavailable
	}
	return resolved, nil
}

func (systemHarnessCommandRunner) RunCommand(
	ctx context.Context,
	request HarnessCommandRequest,
) (HarnessCommandResult, error) {
	defer zeroHarnessBytes(request.Stdin)
	if ctx == nil || !validHarnessCommandRequest(request) {
		return HarnessCommandResult{}, ErrHarnessProcessUnavailable
	}
	before, err := harnessExecutableIdentityFor(request.ExecutablePath)
	if err != nil {
		return HarnessCommandResult{}, ErrHarnessProcessUnavailable
	}
	runContext, cancel := context.WithTimeout(ctx, request.Timeout)
	defer cancel()
	command := exec.CommandContext(
		runContext,
		request.ExecutablePath,
		append([]string(nil), request.Arguments...)...,
	)
	command.Dir = request.Directory
	command.Env = append([]string(nil), request.Environment...)
	command.Stdin = bytes.NewReader(request.Stdin)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		if command.Process == nil {
			return nil
		}
		err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		if errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
	}
	command.WaitDelay = 2 * time.Second
	stdout := &boundedHarnessBuffer{maximum: request.MaxOutputBytes}
	stderr := &boundedHarnessBuffer{maximum: request.MaxOutputBytes}
	command.Stdout = stdout
	command.Stderr = stderr
	launchIdentity, err := harnessExecutableIdentityFor(request.ExecutablePath)
	if err != nil || !sameHarnessExecutableIdentity(before, launchIdentity) {
		return HarnessCommandResult{}, ErrHarnessProcessUnavailable
	}
	runErr := command.Run()
	if runContext.Err() != nil {
		return HarnessCommandResult{}, runContext.Err()
	}
	after, identityErr := harnessExecutableIdentityFor(request.ExecutablePath)
	if identityErr != nil || !sameHarnessExecutableIdentity(before, after) ||
		stdout.overflow || stderr.overflow {
		return HarnessCommandResult{}, ErrHarnessProcessUnavailable
	}
	exitCode := 0
	if runErr != nil {
		var exitError *exec.ExitError
		if !errors.As(runErr, &exitError) {
			return HarnessCommandResult{}, ErrHarnessProcessUnavailable
		}
		exitCode = exitError.ExitCode()
		if exitCode < 0 || exitCode > 255 {
			return HarnessCommandResult{}, ErrHarnessProcessUnavailable
		}
	}
	return HarnessCommandResult{
		Stdout:   append([]byte(nil), stdout.buffer.Bytes()...),
		Stderr:   append([]byte(nil), stderr.buffer.Bytes()...),
		ExitCode: exitCode,
	}, nil
}

func validHarnessCommandRequest(request HarnessCommandRequest) bool {
	if !cleanHarnessAbsolutePath(request.ExecutablePath) ||
		!cleanHarnessAbsolutePath(request.Directory) || request.Timeout <= 0 ||
		request.Timeout > 15*time.Minute || request.MaxOutputBytes < 256 ||
		request.MaxOutputBytes > 1<<20 || len(request.Stdin) > maxHarnessPromptBytes ||
		!utf8.Valid(request.Stdin) || bytes.IndexByte(request.Stdin, 0) >= 0 {
		return false
	}
	for _, argument := range request.Arguments {
		if len(argument) > 1<<20 || strings.IndexByte(argument, 0) >= 0 {
			return false
		}
	}
	for _, value := range request.Environment {
		if value == "" || len(value) > 1<<20 || !utf8.ValidString(value) ||
			strings.IndexByte(value, 0) >= 0 || strings.ContainsAny(value, "\r\n") {
			return false
		}
	}
	info, err := os.Lstat(request.Directory)
	return err == nil && info.IsDir() && info.Mode()&os.ModeSymlink == 0
}

func harnessExecutableIdentityFor(path string) (harnessExecutableIdentity, error) {
	if !cleanHarnessAbsolutePath(path) {
		return harnessExecutableIdentity{}, ErrHarnessProcessUnavailable
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return harnessExecutableIdentity{}, ErrHarnessProcessUnavailable
	}
	directoryPath := filepath.Dir(path)
	directory, err := os.Lstat(directoryPath)
	if err != nil || !directory.IsDir() || directory.Mode()&os.ModeSymlink != 0 {
		return harnessExecutableIdentity{}, ErrHarnessProcessUnavailable
	}
	return harnessExecutableIdentity{
		path: path, directoryPath: directoryPath, file: info, directory: directory,
	}, nil
}

func sameHarnessExecutableIdentity(left, right harnessExecutableIdentity) bool {
	return left.path == right.path && left.directoryPath == right.directoryPath &&
		left.file != nil && right.file != nil && left.directory != nil &&
		right.directory != nil && os.SameFile(left.file, right.file) &&
		os.SameFile(left.directory, right.directory) &&
		left.file.Mode() == right.file.Mode() &&
		left.file.Size() == right.file.Size() &&
		left.file.ModTime() == right.file.ModTime() &&
		left.directory.Mode() == right.directory.Mode()
}

func (buffer *boundedHarnessBuffer) Write(data []byte) (int, error) {
	if buffer.overflow {
		return 0, ErrHarnessProcessUnavailable
	}
	remaining := buffer.maximum - buffer.buffer.Len()
	if remaining <= 0 || len(data) > remaining {
		buffer.overflow = true
		return 0, ErrHarnessProcessUnavailable
	}
	return buffer.buffer.Write(data)
}
