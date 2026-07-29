package provider

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"syscall"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalidCodexNativeAuthConfig   = errors.New("invalid Codex native auth configuration")
	ErrCodexExecutableIdentityChanged = errors.New("Codex executable identity changed")
	errCodexStatusOutputLimit         = errors.New("Codex status output limit")
)

type CodexNativeAuthStatus string

const (
	CodexNativeAuthAvailable   CodexNativeAuthStatus = "available"
	CodexNativeAuthNotLoggedIn CodexNativeAuthStatus = "not_logged_in"
	CodexNativeAuthUnsupported CodexNativeAuthStatus = "unsupported"
	CodexNativeAuthUnavailable CodexNativeAuthStatus = "unavailable"
)

type CodexNativeAuthReason string

const (
	CodexNativeAuthReasonNone            CodexNativeAuthReason = ""
	CodexNativeAuthReasonNotLoggedIn     CodexNativeAuthReason = "not_logged_in"
	CodexNativeAuthReasonUnknownOutput   CodexNativeAuthReason = "unknown_output"
	CodexNativeAuthReasonTimeout         CodexNativeAuthReason = "timeout"
	CodexNativeAuthReasonIdentityChanged CodexNativeAuthReason = "identity_changed"
	CodexNativeAuthReasonUnavailable     CodexNativeAuthReason = "unavailable"
)

type CodexStatusProcessRequest struct {
	ExecutablePath string
	Arguments      []string
	Environment    []string
	MaxOutputBytes int
}

type CodexStatusProcessResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

type CodexStatusRunner interface {
	RunCodexStatus(
		context.Context,
		CodexStatusProcessRequest,
	) (CodexStatusProcessResult, error)
}

type CodexNativeAuthConfig struct {
	ExecutablePath string
	Timeout        time.Duration
	MaxOutputBytes int
	Runner         CodexStatusRunner
}

type CodexNativeAuthObservation struct {
	Status    CodexNativeAuthStatus
	AuthMode  string
	Reason    CodexNativeAuthReason
	RawOutput []byte
}

type CodexNativeAuthObserver struct {
	executablePath string
	timeout        time.Duration
	maxOutputBytes int
	runner         CodexStatusRunner
}

func NewCodexNativeAuthObserver(
	config CodexNativeAuthConfig,
) (*CodexNativeAuthObserver, error) {
	if !filepath.IsAbs(config.ExecutablePath) ||
		config.Timeout <= 0 ||
		config.MaxOutputBytes < 64 ||
		config.MaxOutputBytes > 65536 ||
		interfaceIsNil(config.Runner) {
		return nil, ErrInvalidCodexNativeAuthConfig
	}
	info, err := os.Lstat(config.ExecutablePath)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return nil, ErrInvalidCodexNativeAuthConfig
	}
	return &CodexNativeAuthObserver{
		executablePath: config.ExecutablePath,
		timeout:        config.Timeout,
		maxOutputBytes: config.MaxOutputBytes,
		runner:         config.Runner,
	}, nil
}

func (observer *CodexNativeAuthObserver) Observe(
	ctx context.Context,
) (CodexNativeAuthObservation, error) {
	if observer == nil || ctx == nil {
		return CodexNativeAuthObservation{}, ErrInvalidCodexNativeAuthConfig
	}
	runContext, cancel := context.WithTimeout(ctx, observer.timeout)
	defer cancel()
	result, err := observer.runner.RunCodexStatus(
		runContext,
		CodexStatusProcessRequest{
			ExecutablePath: observer.executablePath,
			Arguments:      []string{"login", "status"},
			Environment:    []string{},
			MaxOutputBytes: observer.maxOutputBytes,
		},
	)
	switch {
	case errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(runContext.Err(), context.DeadlineExceeded):
		return codexObservation(
			CodexNativeAuthUnavailable,
			CodexNativeAuthReasonTimeout,
		), nil
	case errors.Is(err, ErrCodexExecutableIdentityChanged):
		return codexObservation(
			CodexNativeAuthUnavailable,
			CodexNativeAuthReasonIdentityChanged,
		), nil
	case err != nil:
		return codexObservation(
			CodexNativeAuthUnavailable,
			CodexNativeAuthReasonUnavailable,
		), nil
	}
	output, completeLine := codexStatusLine(result.Stdout)
	if len(result.Stderr) != 0 {
		completeLine = false
	}
	switch {
	case completeLine &&
		result.ExitCode == 0 &&
		output == "Logged in using ChatGPT":
		return codexObservation(
			CodexNativeAuthAvailable,
			CodexNativeAuthReasonNone,
		), nil
	case completeLine && output == "Not logged in":
		return codexObservation(
			CodexNativeAuthNotLoggedIn,
			CodexNativeAuthReasonNotLoggedIn,
		), nil
	default:
		return codexObservation(
			CodexNativeAuthUnsupported,
			CodexNativeAuthReasonUnknownOutput,
		), nil
	}
}

func codexStatusLine(output []byte) (string, bool) {
	if !utf8.Valid(output) || len(output) == 0 {
		return "", false
	}
	if bytes.HasSuffix(output, []byte("\r\n")) {
		output = output[:len(output)-2]
	} else if bytes.HasSuffix(output, []byte("\n")) {
		output = output[:len(output)-1]
	}
	if len(output) == 0 ||
		bytes.ContainsAny(output, "\r\n") {
		return "", false
	}
	return string(output), true
}

func codexObservation(
	status CodexNativeAuthStatus,
	reason CodexNativeAuthReason,
) CodexNativeAuthObservation {
	return CodexNativeAuthObservation{
		Status:    status,
		AuthMode:  "native_auth",
		Reason:    reason,
		RawOutput: []byte{},
	}
}

type SystemCodexStatusRunner struct{}

func NewSystemCodexStatusRunner() SystemCodexStatusRunner {
	return SystemCodexStatusRunner{}
}

func (SystemCodexStatusRunner) RunCodexStatus(
	ctx context.Context,
	request CodexStatusProcessRequest,
) (CodexStatusProcessResult, error) {
	before, err := codexExecutableIdentity(request.ExecutablePath)
	if err != nil {
		return CodexStatusProcessResult{}, ErrCodexExecutableIdentityChanged
	}
	if len(request.Arguments) != 2 ||
		request.Arguments[0] != "login" ||
		request.Arguments[1] != "status" ||
		len(request.Environment) != 0 ||
		request.MaxOutputBytes < 64 {
		return CodexStatusProcessResult{}, ErrInvalidCodexNativeAuthConfig
	}
	command := exec.CommandContext(
		ctx,
		request.ExecutablePath,
		request.Arguments...,
	)
	command.Env = []string{}
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
	stdout := &boundedCodexBuffer{maximum: request.MaxOutputBytes}
	stderr := &boundedCodexBuffer{maximum: request.MaxOutputBytes}
	command.Stdout = stdout
	command.Stderr = stderr
	launchIdentity, identityErr := codexExecutableIdentity(
		request.ExecutablePath,
	)
	if identityErr != nil || !sameCodexExecutableIdentity(
		before,
		launchIdentity,
	) {
		return CodexStatusProcessResult{}, ErrCodexExecutableIdentityChanged
	}
	runErr := command.Run()
	after, identityErr := codexExecutableIdentity(request.ExecutablePath)
	if identityErr != nil || !sameCodexExecutableIdentity(before, after) {
		return CodexStatusProcessResult{}, ErrCodexExecutableIdentityChanged
	}
	if errors.Is(stdout.err, errCodexStatusOutputLimit) ||
		errors.Is(stderr.err, errCodexStatusOutputLimit) {
		return CodexStatusProcessResult{}, errCodexStatusOutputLimit
	}
	exitCode := 0
	if runErr != nil {
		if ctx.Err() != nil {
			return CodexStatusProcessResult{}, ctx.Err()
		}
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return CodexStatusProcessResult{}, runErr
		}
	}
	return CodexStatusProcessResult{
		Stdout:   append([]byte(nil), stdout.Bytes()...),
		Stderr:   append([]byte(nil), stderr.Bytes()...),
		ExitCode: exitCode,
	}, nil
}

type codexFileIdentity struct {
	path          string
	directoryPath string
	file          os.FileInfo
	directory     os.FileInfo
}

func codexExecutableIdentity(path string) (codexFileIdentity, error) {
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) || cleaned != path {
		return codexFileIdentity{}, ErrCodexExecutableIdentityChanged
	}
	info, err := os.Lstat(cleaned)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&0o111 == 0 {
		return codexFileIdentity{}, ErrCodexExecutableIdentityChanged
	}
	directoryPath := filepath.Dir(cleaned)
	directory, err := os.Lstat(directoryPath)
	if err != nil || !directory.IsDir() {
		return codexFileIdentity{}, ErrCodexExecutableIdentityChanged
	}
	return codexFileIdentity{
		path:          cleaned,
		directoryPath: directoryPath,
		file:          info,
		directory:     directory,
	}, nil
}

func sameCodexExecutableIdentity(
	left,
	right codexFileIdentity,
) bool {
	return left.path == right.path &&
		left.directoryPath == right.directoryPath &&
		left.file != nil &&
		right.file != nil &&
		left.directory != nil &&
		right.directory != nil &&
		os.SameFile(left.file, right.file) &&
		os.SameFile(left.directory, right.directory) &&
		left.file.Mode() == right.file.Mode() &&
		left.file.Size() == right.file.Size() &&
		left.file.ModTime() == right.file.ModTime() &&
		left.directory.Mode() == right.directory.Mode()
}

type boundedCodexBuffer struct {
	buffer  bytes.Buffer
	maximum int
	err     error
}

func (buffer *boundedCodexBuffer) Write(data []byte) (int, error) {
	if buffer.err != nil {
		return 0, buffer.err
	}
	remaining := buffer.maximum - buffer.buffer.Len()
	if remaining <= 0 || len(data) > remaining {
		buffer.err = errCodexStatusOutputLimit
		return 0, buffer.err
	}
	return buffer.buffer.Write(data)
}

func (buffer *boundedCodexBuffer) Bytes() []byte {
	return buffer.buffer.Bytes()
}

func interfaceIsNil(value any) bool {
	if value == nil {
		return true
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map,
		reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
