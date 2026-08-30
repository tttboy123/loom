package harnessadapter

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

var (
	ErrInvalidClaudeCodeLoginController    = errors.New("invalid Claude Code login controller")
	ErrClaudeCodeLoginUnavailable          = errors.New("Claude Code login unavailable")
	ErrClaudeCodeLoginBusy                 = errors.New("Claude Code login already running")
	ErrClaudeCodeExecutableIdentityChanged = errors.New("Claude Code executable identity changed")
)

type ClaudeCodeLoginController interface {
	Start(context.Context) error
	Cancel() error
	Close() error
}

type ClaudeCodeLoginControllerConfig struct {
	ExecutablePath string
	HomePath       string
	TempPath       string
	Timeout        time.Duration
}

type claudeCodeLoginDirectoryIdentity struct {
	path string
	info os.FileInfo
}

type SystemClaudeCodeLoginController struct {
	executable harnessExecutableIdentity
	home       claudeCodeLoginDirectoryIdentity
	temp       claudeCodeLoginDirectoryIdentity
	timeout    time.Duration

	mu      sync.Mutex
	running bool
	closed  bool
	cancel  context.CancelFunc
	done    chan struct{}
}

func NewSystemClaudeCodeLoginController(
	config ClaudeCodeLoginControllerConfig,
) (*SystemClaudeCodeLoginController, error) {
	if config.Timeout <= 0 || config.Timeout > 15*time.Minute {
		return nil, ErrInvalidClaudeCodeLoginController
	}
	executable, err := harnessExecutableIdentityFor(config.ExecutablePath)
	if err != nil {
		return nil, ErrInvalidClaudeCodeLoginController
	}
	home, err := claudeCodeLoginDirectoryIdentityFor(config.HomePath)
	if err != nil {
		return nil, ErrInvalidClaudeCodeLoginController
	}
	temp, err := claudeCodeLoginDirectoryIdentityFor(config.TempPath)
	if err != nil {
		return nil, ErrInvalidClaudeCodeLoginController
	}
	return &SystemClaudeCodeLoginController{
		executable: executable, home: home, temp: temp, timeout: config.Timeout,
	}, nil
}

func (controller *SystemClaudeCodeLoginController) Start(ctx context.Context) error {
	if controller == nil || ctx == nil {
		return ErrInvalidClaudeCodeLoginController
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	controller.mu.Lock()
	defer controller.mu.Unlock()
	if controller.closed {
		return ErrClaudeCodeLoginUnavailable
	}
	if controller.running {
		return ErrClaudeCodeLoginBusy
	}
	if !controller.identitiesCurrent() {
		return ErrClaudeCodeExecutableIdentityChanged
	}
	runContext, cancel := context.WithTimeout(context.Background(), controller.timeout)
	command := exec.CommandContext(
		runContext, controller.executable.path, "auth", "login",
	)
	command.Dir = controller.home.path
	command.Env = claudeCodeBaseEnvironmentFor(
		controller.executable.path, controller.home.path, controller.temp.path,
	)
	// The official auth flow waits for Enter before opening the browser. The
	// App's explicit Sign In action supplies only that fixed acknowledgement.
	command.Stdin = bytes.NewReader([]byte{'\n'})
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
	if !controller.identitiesCurrent() {
		cancel()
		return ErrClaudeCodeExecutableIdentityChanged
	}
	if err := command.Start(); err != nil {
		cancel()
		return ErrClaudeCodeLoginUnavailable
	}
	if !controller.identitiesCurrent() {
		cancel()
		_ = command.Wait()
		return ErrClaudeCodeExecutableIdentityChanged
	}
	controller.running = true
	controller.cancel = cancel
	controller.done = make(chan struct{})
	done := controller.done
	go controller.wait(command, cancel, done)
	return nil
}

func (controller *SystemClaudeCodeLoginController) identitiesCurrent() bool {
	currentExecutable, executableErr := harnessExecutableIdentityFor(
		controller.executable.path,
	)
	currentHome, homeErr := claudeCodeLoginDirectoryIdentityFor(controller.home.path)
	currentTemp, tempErr := claudeCodeLoginDirectoryIdentityFor(controller.temp.path)
	return executableErr == nil && homeErr == nil && tempErr == nil &&
		sameHarnessExecutableIdentity(controller.executable, currentExecutable) &&
		sameClaudeCodeLoginDirectoryIdentity(controller.home, currentHome) &&
		sameClaudeCodeLoginDirectoryIdentity(controller.temp, currentTemp)
}

func (controller *SystemClaudeCodeLoginController) wait(
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

func (controller *SystemClaudeCodeLoginController) Cancel() error {
	if controller == nil {
		return ErrInvalidClaudeCodeLoginController
	}
	controller.mu.Lock()
	if controller.closed {
		controller.mu.Unlock()
		return ErrClaudeCodeLoginUnavailable
	}
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

func (controller *SystemClaudeCodeLoginController) Close() error {
	if controller == nil {
		return ErrInvalidClaudeCodeLoginController
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

func claudeCodeLoginDirectoryIdentityFor(
	path string,
) (claudeCodeLoginDirectoryIdentity, error) {
	cleaned := filepath.Clean(path)
	if !filepath.IsAbs(cleaned) || cleaned != path {
		return claudeCodeLoginDirectoryIdentity{}, ErrInvalidClaudeCodeLoginController
	}
	info, err := os.Lstat(cleaned)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return claudeCodeLoginDirectoryIdentity{}, ErrInvalidClaudeCodeLoginController
	}
	return claudeCodeLoginDirectoryIdentity{path: cleaned, info: info}, nil
}

func sameClaudeCodeLoginDirectoryIdentity(
	left claudeCodeLoginDirectoryIdentity,
	right claudeCodeLoginDirectoryIdentity,
) bool {
	return left.path == right.path && left.info != nil && right.info != nil &&
		os.SameFile(left.info, right.info) && left.info.Mode() == right.info.Mode()
}
