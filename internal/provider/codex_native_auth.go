package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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
	errCodexAuthProbeOutputLimit      = errors.New("Codex auth probe output limit")
	errCodexAuthProbeProtocol         = errors.New("Codex auth probe protocol failure")
)

const (
	codexAuthProbeInitializeID = "loom-auth-initialize-v1"
	codexAuthProbeAccountID    = "loom-auth-account-read-v1"
	codexAuthProbeMaximumLines = 256
	codexAuthAvailableCacheTTL = 30 * time.Second
	codexAuthRetryCacheTTL     = 2 * time.Second
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

type CodexNativeAuthProbeRequest struct {
	ExecutablePath string
	MaxOutputBytes int
}

type CodexNativeAuthProbeResult struct {
	Authenticated bool
	// RequiresOpenAIAuth describes the active model Provider. It is true for
	// an authenticated ChatGPT/OpenAI account, not a missing-auth signal.
	RequiresOpenAIAuth bool
}

type CodexNativeAuthProbeRunner interface {
	ProbeCodexNativeAuth(
		context.Context,
		CodexNativeAuthProbeRequest,
	) (CodexNativeAuthProbeResult, error)
}

type CodexNativeAuthConfig struct {
	ExecutablePath string
	Timeout        time.Duration
	MaxOutputBytes int
	Runner         CodexNativeAuthProbeRunner
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
	runner         CodexNativeAuthProbeRunner

	mu          sync.Mutex
	cached      CodexNativeAuthObservation
	cacheUntil  time.Time
	cacheExists bool
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
	if observer == nil || ctx == nil || ctx.Err() != nil {
		return CodexNativeAuthObservation{}, ErrInvalidCodexNativeAuthConfig
	}
	observer.mu.Lock()
	defer observer.mu.Unlock()
	if ctx.Err() != nil {
		return CodexNativeAuthObservation{}, ctx.Err()
	}
	if observer.cacheExists && time.Now().Before(observer.cacheUntil) {
		return observer.cached, nil
	}
	result, err := observer.observeUncached(ctx)
	if err != nil {
		return CodexNativeAuthObservation{}, err
	}
	ttl := codexAuthRetryCacheTTL
	if result.Status == CodexNativeAuthAvailable || result.Status == CodexNativeAuthUnsupported {
		ttl = codexAuthAvailableCacheTTL
	}
	observer.cached = result
	observer.cacheUntil = time.Now().Add(ttl)
	observer.cacheExists = true
	return result, nil
}

func (observer *CodexNativeAuthObserver) Invalidate() {
	if observer == nil {
		return
	}
	observer.mu.Lock()
	observer.cached = CodexNativeAuthObservation{}
	observer.cacheUntil = time.Time{}
	observer.cacheExists = false
	observer.mu.Unlock()
}

func (observer *CodexNativeAuthObserver) observeUncached(
	ctx context.Context,
) (CodexNativeAuthObservation, error) {
	runContext, cancel := context.WithTimeout(ctx, observer.timeout)
	defer cancel()
	result, err := observer.runner.ProbeCodexNativeAuth(
		runContext,
		CodexNativeAuthProbeRequest{
			ExecutablePath: observer.executablePath,
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
	switch {
	case result.Authenticated && result.RequiresOpenAIAuth:
		return codexObservation(
			CodexNativeAuthAvailable,
			CodexNativeAuthReasonNone,
		), nil
	case !result.Authenticated && result.RequiresOpenAIAuth:
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

type SystemCodexNativeAuthProbeRunner struct{}

func NewSystemCodexNativeAuthProbeRunner() SystemCodexNativeAuthProbeRunner {
	return SystemCodexNativeAuthProbeRunner{}
}

func (SystemCodexNativeAuthProbeRunner) ProbeCodexNativeAuth(
	ctx context.Context,
	request CodexNativeAuthProbeRequest,
) (CodexNativeAuthProbeResult, error) {
	if ctx == nil || ctx.Err() != nil {
		return CodexNativeAuthProbeResult{}, ErrInvalidCodexNativeAuthConfig
	}
	before, err := codexExecutableIdentity(request.ExecutablePath)
	if err != nil {
		return CodexNativeAuthProbeResult{}, ErrCodexExecutableIdentityChanged
	}
	if request.MaxOutputBytes < 64 || request.MaxOutputBytes > 65_536 {
		return CodexNativeAuthProbeResult{}, ErrInvalidCodexNativeAuthConfig
	}
	command := exec.CommandContext(
		ctx,
		request.ExecutablePath,
		"-c", "mcp_servers={}", "app-server", "--stdio",
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
	stdin, err := command.StdinPipe()
	if err != nil {
		return CodexNativeAuthProbeResult{}, errCodexAuthProbeProtocol
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return CodexNativeAuthProbeResult{}, errCodexAuthProbeProtocol
	}
	stderr := &boundedCodexDiscard{maximum: request.MaxOutputBytes}
	command.Stderr = stderr
	launchIdentity, identityErr := codexExecutableIdentity(
		request.ExecutablePath,
	)
	if identityErr != nil || !sameCodexExecutableIdentity(
		before,
		launchIdentity,
	) {
		return CodexNativeAuthProbeResult{}, ErrCodexExecutableIdentityChanged
	}
	if err := command.Start(); err != nil {
		return CodexNativeAuthProbeResult{}, errCodexAuthProbeProtocol
	}
	waited := false
	defer func() {
		if waited {
			return
		}
		_ = stdin.Close()
		if command.Process != nil {
			_ = syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
		}
		_ = command.Wait()
	}()
	startedIdentity, identityErr := codexExecutableIdentity(request.ExecutablePath)
	if identityErr != nil || !sameCodexExecutableIdentity(before, startedIdentity) {
		return CodexNativeAuthProbeResult{}, ErrCodexExecutableIdentityChanged
	}
	reader := bufio.NewReaderSize(stdout, request.MaxOutputBytes+1)
	consumed := 0
	if err := writeCodexAuthProbeRequest(stdin, map[string]any{
		"id": codexAuthProbeInitializeID, "method": "initialize",
		"params": map[string]any{
			"clientInfo":   map[string]string{"name": "loom", "version": "phase7-auth-v1"},
			"capabilities": map[string]bool{"experimentalApi": true},
		},
	}); err != nil {
		return CodexNativeAuthProbeResult{}, err
	}
	initialize, err := readCodexAuthProbeResponse(
		ctx, reader, codexAuthProbeInitializeID, request.MaxOutputBytes, &consumed,
	)
	if err != nil {
		return CodexNativeAuthProbeResult{}, err
	}
	defer zeroCodexAuthProbeBytes(initialize)
	var initialized struct {
		UserAgent      string `json:"userAgent"`
		CodexHome      string `json:"codexHome"`
		PlatformFamily string `json:"platformFamily"`
		PlatformOS     string `json:"platformOs"`
	}
	if json.Unmarshal(initialize, &initialized) != nil ||
		!strings.Contains(initialized.UserAgent, "0.144.1") ||
		initialized.CodexHome == "" || initialized.PlatformFamily == "" ||
		initialized.PlatformOS == "" {
		return CodexNativeAuthProbeResult{}, errCodexAuthProbeProtocol
	}
	if err := writeCodexAuthProbeRequest(stdin, map[string]any{
		"id": codexAuthProbeAccountID, "method": "account/read",
		"params": map[string]bool{"refreshToken": true},
	}); err != nil {
		return CodexNativeAuthProbeResult{}, err
	}
	accountResult, err := readCodexAuthProbeResponse(
		ctx, reader, codexAuthProbeAccountID, request.MaxOutputBytes, &consumed,
	)
	if err != nil {
		return CodexNativeAuthProbeResult{}, err
	}
	defer zeroCodexAuthProbeBytes(accountResult)
	var account struct {
		Account            json.RawMessage `json:"account"`
		RequiresOpenAIAuth *bool           `json:"requiresOpenaiAuth"`
	}
	if json.Unmarshal(accountResult, &account) != nil || account.RequiresOpenAIAuth == nil {
		return CodexNativeAuthProbeResult{}, errCodexAuthProbeProtocol
	}
	defer zeroCodexAuthProbeBytes(account.Account)
	trimmedAccount := bytes.TrimSpace(account.Account)
	authenticated := len(trimmedAccount) >= 2 && !bytes.Equal(trimmedAccount, []byte("null"))
	if authenticated && (trimmedAccount[0] != '{' ||
		trimmedAccount[len(trimmedAccount)-1] != '}' || !json.Valid(trimmedAccount)) {
		return CodexNativeAuthProbeResult{}, errCodexAuthProbeProtocol
	}
	if err := stdin.Close(); err != nil {
		return CodexNativeAuthProbeResult{}, errCodexAuthProbeProtocol
	}
	runErr := command.Wait()
	waited = true
	after, identityErr := codexExecutableIdentity(request.ExecutablePath)
	if identityErr != nil || !sameCodexExecutableIdentity(before, after) {
		return CodexNativeAuthProbeResult{}, ErrCodexExecutableIdentityChanged
	}
	if errors.Is(stderr.err, errCodexAuthProbeOutputLimit) {
		return CodexNativeAuthProbeResult{}, errCodexAuthProbeOutputLimit
	}
	if runErr != nil {
		if ctx.Err() != nil {
			return CodexNativeAuthProbeResult{}, ctx.Err()
		}
		return CodexNativeAuthProbeResult{}, errCodexAuthProbeProtocol
	}
	return CodexNativeAuthProbeResult{
		Authenticated:      authenticated,
		RequiresOpenAIAuth: *account.RequiresOpenAIAuth,
	}, nil
}

func writeCodexAuthProbeRequest(writer io.Writer, request any) error {
	payload, err := json.Marshal(request)
	if err != nil || len(payload) == 0 || len(payload) > 4_096 {
		zeroCodexAuthProbeBytes(payload)
		return errCodexAuthProbeProtocol
	}
	payload = append(payload, '\n')
	defer zeroCodexAuthProbeBytes(payload)
	if _, err := writer.Write(payload); err != nil {
		return errCodexAuthProbeProtocol
	}
	return nil
}

func readCodexAuthProbeResponse(
	ctx context.Context,
	reader *bufio.Reader,
	expectedID string,
	maximum int,
	consumed *int,
) (json.RawMessage, error) {
	if ctx == nil || reader == nil || expectedID == "" || maximum < 64 || consumed == nil {
		return nil, ErrInvalidCodexNativeAuthConfig
	}
	for count := 0; count < codexAuthProbeMaximumLines; count++ {
		line, err := reader.ReadSlice('\n')
		if err != nil {
			zeroCodexAuthProbeBytes(line)
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if errors.Is(err, bufio.ErrBufferFull) || *consumed+len(line) > maximum {
				return nil, errCodexAuthProbeOutputLimit
			}
			return nil, errCodexAuthProbeProtocol
		}
		*consumed += len(line)
		if *consumed > maximum {
			zeroCodexAuthProbeBytes(line)
			return nil, errCodexAuthProbeOutputLimit
		}
		line = bytes.TrimSuffix(line, []byte{'\n'})
		line = bytes.TrimSuffix(line, []byte{'\r'})
		if len(line) == 0 || !utf8.Valid(line) || bytes.IndexByte(line, 0) >= 0 {
			zeroCodexAuthProbeBytes(line)
			return nil, errCodexAuthProbeProtocol
		}
		var envelope struct {
			ID     string          `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
			Result json.RawMessage `json:"result"`
			Error  json.RawMessage `json:"error"`
		}
		decodeErr := json.Unmarshal(line, &envelope)
		zeroCodexAuthProbeBytes(line)
		if decodeErr != nil {
			zeroCodexAuthProbeBytes(envelope.Params)
			zeroCodexAuthProbeBytes(envelope.Result)
			zeroCodexAuthProbeBytes(envelope.Error)
			return nil, errCodexAuthProbeProtocol
		}
		if envelope.ID == expectedID {
			if envelope.Method != "" || len(envelope.Params) != 0 ||
				len(envelope.Error) != 0 || len(envelope.Result) == 0 {
				zeroCodexAuthProbeBytes(envelope.Params)
				zeroCodexAuthProbeBytes(envelope.Result)
				zeroCodexAuthProbeBytes(envelope.Error)
				return nil, errCodexAuthProbeProtocol
			}
			result := append(json.RawMessage(nil), envelope.Result...)
			zeroCodexAuthProbeBytes(envelope.Result)
			return result, nil
		}
		zeroCodexAuthProbeBytes(envelope.Result)
		zeroCodexAuthProbeBytes(envelope.Error)
		if envelope.ID != "" || envelope.Method == "" || len(envelope.Params) == 0 {
			zeroCodexAuthProbeBytes(envelope.Params)
			return nil, errCodexAuthProbeProtocol
		}
		zeroCodexAuthProbeBytes(envelope.Params)
	}
	return nil, errCodexAuthProbeOutputLimit
}

func zeroCodexAuthProbeBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
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

type boundedCodexDiscard struct {
	written int
	maximum int
	err     error
}

func (writer *boundedCodexDiscard) Write(data []byte) (int, error) {
	if writer.err != nil {
		return 0, writer.err
	}
	remaining := writer.maximum - writer.written
	if remaining <= 0 || len(data) > remaining {
		writer.err = errCodexAuthProbeOutputLimit
		return 0, writer.err
	}
	writer.written += len(data)
	return len(data), nil
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
		buffer.err = errCodexAuthProbeOutputLimit
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
