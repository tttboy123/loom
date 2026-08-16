package harnessadapter

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"
)

type systemHarnessSessionRunner struct{}

type systemHarnessStreamSession struct {
	command  *exec.Cmd
	identity harnessExecutableIdentity
	path     string
	runCtx   context.Context
	cancel   context.CancelFunc
	stdin    io.WriteCloser
	stderr   *boundedHarnessBuffer
	lines    chan []byte

	writeMu sync.Mutex
	stateMu sync.Mutex

	streamDone  chan struct{}
	streamErr   error
	processDone chan struct{}
	processErr  error

	closeOnce sync.Once
	closeErr  error
	abortOnce sync.Once
	abortErr  error
}

func NewSystemHarnessSessionRunner() HarnessSessionRunner {
	return systemHarnessSessionRunner{}
}

func (systemHarnessSessionRunner) StartSession(
	ctx context.Context,
	request HarnessSessionRequest,
) (HarnessStreamSession, error) {
	if ctx == nil || !validHarnessSessionRequest(request) {
		return nil, ErrHarnessProcessUnavailable
	}
	before, err := harnessExecutableIdentityFor(request.ExecutablePath)
	if err != nil {
		return nil, ErrHarnessProcessUnavailable
	}
	runCtx, cancel := context.WithTimeout(ctx, request.Timeout)
	command := exec.CommandContext(
		runCtx,
		request.ExecutablePath,
		append([]string(nil), request.Arguments...)...,
	)
	command.Dir = request.Directory
	command.Env = append([]string(nil), request.Environment...)
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	command.Cancel = func() error {
		return killHarnessProcessGroup(command)
	}
	command.WaitDelay = 2 * time.Second
	stdin, err := command.StdinPipe()
	if err != nil {
		cancel()
		return nil, ErrHarnessProcessUnavailable
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		cancel()
		_ = stdin.Close()
		return nil, ErrHarnessProcessUnavailable
	}
	stderr := &boundedHarnessBuffer{maximum: request.MaxOutputBytes}
	command.Stderr = stderr
	launchIdentity, err := harnessExecutableIdentityFor(request.ExecutablePath)
	if err != nil || !sameHarnessExecutableIdentity(before, launchIdentity) {
		cancel()
		_ = stdin.Close()
		return nil, ErrHarnessProcessUnavailable
	}
	if err := command.Start(); err != nil {
		cancel()
		_ = stdin.Close()
		return nil, ErrHarnessProcessUnavailable
	}
	startedIdentity, err := harnessExecutableIdentityFor(request.ExecutablePath)
	if err != nil || !sameHarnessExecutableIdentity(before, startedIdentity) {
		_ = killHarnessProcessGroup(command)
		cancel()
		_ = stdin.Close()
		_ = command.Wait()
		return nil, ErrHarnessProcessUnavailable
	}
	session := &systemHarnessStreamSession{
		command: command, identity: before, path: request.ExecutablePath,
		runCtx: runCtx, cancel: cancel, stdin: stdin, stderr: stderr,
		lines:      make(chan []byte, 64),
		streamDone: make(chan struct{}), processDone: make(chan struct{}),
	}
	go session.readStdout(stdout, request.MaxOutputBytes)
	go session.waitProcess()
	return session, nil
}

func validHarnessSessionRequest(request HarnessSessionRequest) bool {
	return validHarnessCommandRequest(HarnessCommandRequest{
		ExecutablePath: request.ExecutablePath,
		Arguments:      request.Arguments,
		Environment:    request.Environment,
		Directory:      request.Directory,
		MaxOutputBytes: request.MaxOutputBytes,
		Timeout:        request.Timeout,
	})
}

func (session *systemHarnessStreamSession) WriteLine(
	ctx context.Context,
	payload []byte,
) error {
	defer zeroHarnessBytes(payload)
	if session == nil || ctx == nil || len(payload) == 0 || len(payload) > 1<<20 ||
		!utf8.Valid(payload) || bytes.IndexByte(payload, 0) >= 0 ||
		bytes.IndexAny(payload, "\r\n") >= 0 {
		return ErrHarnessProcessUnavailable
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := session.runCtx.Err(); err != nil {
		return err
	}
	wire := make([]byte, len(payload)+1)
	copy(wire, payload)
	wire[len(payload)] = '\n'
	defer zeroHarnessBytes(wire)
	session.writeMu.Lock()
	defer session.writeMu.Unlock()
	if err := session.runCtx.Err(); err != nil {
		return err
	}
	if _, err := session.stdin.Write(wire); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if runErr := session.runCtx.Err(); runErr != nil {
			return runErr
		}
		return ErrHarnessProcessUnavailable
	}
	return nil
}

func (session *systemHarnessStreamSession) ReadLine(ctx context.Context) ([]byte, error) {
	if session == nil || ctx == nil {
		return nil, ErrHarnessProcessUnavailable
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case line, ok := <-session.lines:
		if ok {
			return line, nil
		}
		session.stateMu.Lock()
		err := session.streamErr
		session.stateMu.Unlock()
		if err != nil {
			return nil, err
		}
		return nil, io.EOF
	}
}

func (session *systemHarnessStreamSession) CloseInput() error {
	if session == nil {
		return ErrHarnessProcessUnavailable
	}
	session.closeOnce.Do(func() {
		session.writeMu.Lock()
		defer session.writeMu.Unlock()
		session.closeErr = session.stdin.Close()
	})
	if errors.Is(session.closeErr, syscall.EBADF) {
		return nil
	}
	return session.closeErr
}

func (session *systemHarnessStreamSession) Wait(
	ctx context.Context,
) (HarnessCommandResult, error) {
	if session == nil || ctx == nil {
		return HarnessCommandResult{}, ErrHarnessProcessUnavailable
	}
	select {
	case <-ctx.Done():
		_ = session.Abort()
		return HarnessCommandResult{}, ctx.Err()
	case <-session.processDone:
	}
	select {
	case <-ctx.Done():
		_ = session.Abort()
		return HarnessCommandResult{}, ctx.Err()
	case <-session.streamDone:
	}
	defer session.cancel()
	if err := session.runCtx.Err(); err != nil {
		return HarnessCommandResult{}, err
	}
	session.stateMu.Lock()
	processErr := session.processErr
	streamErr := session.streamErr
	session.stateMu.Unlock()
	after, identityErr := harnessExecutableIdentityFor(session.path)
	if identityErr != nil || !sameHarnessExecutableIdentity(session.identity, after) ||
		streamErr != nil || session.stderr.overflow {
		return HarnessCommandResult{}, ErrHarnessProcessUnavailable
	}
	exitCode := 0
	if processErr != nil {
		var exitError *exec.ExitError
		if !errors.As(processErr, &exitError) {
			return HarnessCommandResult{}, ErrHarnessProcessUnavailable
		}
		exitCode = exitError.ExitCode()
		if exitCode < 0 || exitCode > 255 {
			return HarnessCommandResult{}, ErrHarnessProcessUnavailable
		}
	}
	return HarnessCommandResult{
		Stderr:   append([]byte(nil), session.stderr.buffer.Bytes()...),
		ExitCode: exitCode,
	}, nil
}

func (session *systemHarnessStreamSession) Abort() error {
	if session == nil {
		return ErrHarnessProcessUnavailable
	}
	session.abortOnce.Do(func() {
		session.cancel()
		closeErr := session.CloseInput()
		killErr := killHarnessProcessGroup(session.command)
		select {
		case <-session.processDone:
		case <-time.After(2 * time.Second):
			session.abortErr = ErrHarnessProcessUnavailable
			return
		}
		if closeErr != nil && !errors.Is(closeErr, syscall.EBADF) {
			session.abortErr = closeErr
		}
		if killErr != nil {
			session.abortErr = errors.Join(session.abortErr, killErr)
		}
	})
	return session.abortErr
}

func (session *systemHarnessStreamSession) readStdout(
	stdout io.ReadCloser,
	maximum int,
) {
	defer close(session.streamDone)
	defer close(session.lines)
	defer stdout.Close()
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 4096), maximum+1)
	total := 0
	for scanner.Scan() {
		line := bytes.Clone(scanner.Bytes())
		total += len(line) + 1
		if len(line) == 0 || len(line) > maximum || total > maximum ||
			!utf8.Valid(line) || bytes.IndexByte(line, 0) >= 0 {
			zeroHarnessBytes(line)
			session.setStreamError(ErrHarnessProcessUnavailable)
			_ = killHarnessProcessGroup(session.command)
			return
		}
		select {
		case session.lines <- line:
		case <-session.runCtx.Done():
			zeroHarnessBytes(line)
			session.setStreamError(session.runCtx.Err())
			return
		}
	}
	if scanner.Err() != nil {
		session.setStreamError(ErrHarnessProcessUnavailable)
		_ = killHarnessProcessGroup(session.command)
	}
}

func (session *systemHarnessStreamSession) setStreamError(err error) {
	session.stateMu.Lock()
	defer session.stateMu.Unlock()
	session.streamErr = err
}

func (session *systemHarnessStreamSession) waitProcess() {
	err := session.command.Wait()
	session.stateMu.Lock()
	session.processErr = err
	session.stateMu.Unlock()
	close(session.processDone)
}

func killHarnessProcessGroup(command *exec.Cmd) error {
	if command == nil || command.Process == nil {
		return nil
	}
	err := syscall.Kill(-command.Process.Pid, syscall.SIGKILL)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}
