package provider

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
)

// ResolveCodexNativeExecutable converts a supported launcher into the exact
// regular native executable whose identity the auth runners will fence.
func ResolveCodexNativeExecutable(path string) (string, error) {
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) || cleaned != path {
		return "", ErrInvalidCodexNativeAuthConfig
	}
	info, err := os.Lstat(cleaned)
	if err != nil {
		return "", ErrInvalidCodexNativeAuthConfig
	}
	if info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
		return cleaned, nil
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return "", ErrInvalidCodexNativeAuthConfig
	}
	wrapper, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		return "", ErrInvalidCodexNativeAuthConfig
	}
	wrapper = filepath.Clean(wrapper)
	wrapperInfo, err := os.Lstat(wrapper)
	if err != nil || !wrapperInfo.Mode().IsRegular() || wrapperInfo.Mode()&0o111 == 0 ||
		filepath.Base(wrapper) != "codex.js" {
		return "", ErrInvalidCodexNativeAuthConfig
	}
	packageRoot := filepath.Dir(filepath.Dir(wrapper))
	if filepath.Base(packageRoot) != "codex" ||
		filepath.Base(filepath.Dir(packageRoot)) != "@openai" {
		return "", ErrInvalidCodexNativeAuthConfig
	}
	triple, platformPackage := "", ""
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "darwin/arm64":
		triple, platformPackage = "aarch64-apple-darwin", "codex-darwin-arm64"
	case "darwin/amd64":
		triple, platformPackage = "x86_64-apple-darwin", "codex-darwin-x64"
	default:
		return "", ErrInvalidCodexNativeAuthConfig
	}
	candidates := []string{
		filepath.Join(packageRoot, "node_modules", "@openai", platformPackage, "vendor", triple, "bin", "codex"),
		filepath.Join(packageRoot, "vendor", triple, "bin", "codex"),
	}
	for _, candidate := range candidates {
		candidateInfo, candidateErr := os.Lstat(candidate)
		if candidateErr != nil || !candidateInfo.Mode().IsRegular() || candidateInfo.Mode()&0o111 == 0 {
			continue
		}
		resolved, resolveErr := filepath.EvalSymlinks(candidate)
		if resolveErr != nil {
			continue
		}
		resolved = filepath.Clean(resolved)
		resolvedInfo, statErr := os.Lstat(resolved)
		relative, relativeErr := filepath.Rel(packageRoot, resolved)
		if statErr == nil && relativeErr == nil && relative != ".." &&
			!strings.HasPrefix(relative, ".."+string(filepath.Separator)) && filepath.IsAbs(resolved) &&
			resolvedInfo.Mode().IsRegular() && resolvedInfo.Mode()&0o111 != 0 {
			return resolved, nil
		}
	}
	return "", ErrInvalidCodexNativeAuthConfig
}

var (
	ErrInvalidCodexNativeAuthConfig   = errors.New("invalid Codex native auth configuration")
	ErrCodexExecutableIdentityChanged = errors.New("Codex executable identity changed")
	ErrInvalidCodexLoginController    = errors.New("invalid Codex login controller")
	ErrCodexLoginBusy                 = errors.New("Codex login already running")
	ErrCodexLoginUnavailable          = errors.New("Codex login unavailable")
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

type CodexLoginStatus string

const CodexLoginStarted CodexLoginStatus = "started"

type CodexLoginStartResult struct {
	Status CodexLoginStatus `json:"status"`
}

type CodexLoginController interface {
	Start(context.Context) (CodexLoginStartResult, error)
	Close() error
}

type CodexLoginControllerConfig struct {
	ExecutablePath string
	Timeout        time.Duration
}

type SystemCodexLoginController struct {
	executable codexFileIdentity
	timeout    time.Duration

	mu      sync.Mutex
	running bool
	closed  bool
	cancel  context.CancelFunc
	done    chan struct{}
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
	output, completeLine := codexStatusOutput(result.Stdout, result.Stderr)
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

func codexStatusOutput(stdout, stderr []byte) (string, bool) {
	switch {
	case len(stdout) != 0 && len(stderr) == 0:
		return codexStatusLine(stdout)
	case len(stderr) != 0 && len(stdout) == 0:
		return codexStatusLine(stderr)
	default:
		return "", false
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

func NewSystemCodexLoginController(
	config CodexLoginControllerConfig,
) (*SystemCodexLoginController, error) {
	if config.Timeout <= 0 || config.Timeout > 15*time.Minute {
		return nil, ErrInvalidCodexLoginController
	}
	executable, err := codexExecutableIdentity(config.ExecutablePath)
	if err != nil {
		return nil, ErrInvalidCodexLoginController
	}
	return &SystemCodexLoginController{
		executable: executable,
		timeout:    config.Timeout,
	}, nil
}

func (controller *SystemCodexLoginController) Start(
	ctx context.Context,
) (CodexLoginStartResult, error) {
	if controller == nil || ctx == nil {
		return CodexLoginStartResult{}, ErrInvalidCodexLoginController
	}
	if err := ctx.Err(); err != nil {
		return CodexLoginStartResult{}, err
	}
	controller.mu.Lock()
	defer controller.mu.Unlock()
	if controller.closed {
		return CodexLoginStartResult{}, ErrCodexLoginUnavailable
	}
	if controller.running {
		return CodexLoginStartResult{}, ErrCodexLoginBusy
	}
	current, err := codexExecutableIdentity(controller.executable.path)
	if err != nil || !sameCodexExecutableIdentity(
		controller.executable,
		current,
	) {
		return CodexLoginStartResult{},
			ErrCodexExecutableIdentityChanged
	}
	runContext, cancel := context.WithTimeout(
		context.Background(),
		controller.timeout,
	)
	command := exec.CommandContext(
		runContext,
		controller.executable.path,
		"login",
	)
	command.Env = []string{}
	command.Stdout = io.Discard
	command.Stderr = io.Discard
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
	launchIdentity, identityErr := codexExecutableIdentity(
		controller.executable.path,
	)
	if identityErr != nil || !sameCodexExecutableIdentity(
		controller.executable,
		launchIdentity,
	) {
		cancel()
		return CodexLoginStartResult{},
			ErrCodexExecutableIdentityChanged
	}
	if err := command.Start(); err != nil {
		cancel()
		return CodexLoginStartResult{}, ErrCodexLoginUnavailable
	}
	controller.running = true
	controller.cancel = cancel
	controller.done = make(chan struct{})
	done := controller.done
	go controller.wait(command, cancel, done)
	return CodexLoginStartResult{Status: CodexLoginStarted}, nil
}

func (controller *SystemCodexLoginController) wait(
	command *exec.Cmd,
	cancel context.CancelFunc,
	done chan struct{},
) {
	_ = command.Wait()
	cancel()
	controller.mu.Lock()
	controller.running = false
	controller.cancel = nil
	if controller.done == done {
		controller.done = nil
	}
	close(done)
	controller.mu.Unlock()
}

func (controller *SystemCodexLoginController) Close() error {
	if controller == nil {
		return ErrInvalidCodexLoginController
	}
	controller.mu.Lock()
	if controller.closed {
		controller.mu.Unlock()
		return nil
	}
	controller.closed = true
	cancel := controller.cancel
	done := controller.done
	controller.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		<-done
	}
	return nil
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
