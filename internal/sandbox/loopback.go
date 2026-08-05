package sandbox

// LoopbackBackend is the Phase 3B controlled spike backend. It provides
// process-level isolation for the canary: a fresh per-instance temp
// workspace, a sanitized environment that never carries provider secrets,
// a bounded timeout, and real cancel/pause/resume/destroy. It is a fixture
// for deterministic journeys — not a production isolation boundary.

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

const loopbackOutputLimit = 64 << 10

type LoopbackBackend struct {
	mu        sync.Mutex
	workRoot  string
	timeout   time.Duration
	dirs      map[string]string
	procs     map[string]*os.Process
	destroyed []string
}

func NewLoopbackBackend(workRoot string, timeout time.Duration) (*LoopbackBackend, error) {
	if workRoot == "" {
		workRoot = os.TempDir()
	}
	if err := os.MkdirAll(workRoot, 0o700); err != nil {
		return nil, err
	}
	root, err := filepath.EvalSymlinks(workRoot)
	if err != nil {
		return nil, err
	}
	return &LoopbackBackend{
		workRoot: root, timeout: timeout,
		dirs: make(map[string]string), procs: make(map[string]*os.Process),
	}, nil
}

func (backend *LoopbackBackend) Create(ctx context.Context, request CreateRequest) (Instance, error) {
	instanceID := deterministicInstanceID(request.JobID, request.RunID, request.Generation)
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if _, exists := backend.dirs[instanceID]; exists {
		return Instance{InstanceID: instanceID}, nil
	}
	workdir := filepath.Join(backend.workRoot, instanceID)
	if err := os.MkdirAll(workdir, 0o700); err != nil {
		return Instance{}, err
	}
	if err := os.MkdirAll(filepath.Join(workdir, "tmp"), 0o700); err != nil {
		return Instance{}, err
	}
	if err := os.Chmod(workdir, 0o700); err != nil {
		return Instance{}, err
	}
	backend.dirs[instanceID] = workdir
	return Instance{InstanceID: instanceID}, nil
}

func (backend *LoopbackBackend) Exec(ctx context.Context, request ExecRequest) (ExecResult, error) {
	backend.mu.Lock()
	workdir, ok := backend.dirs[request.InstanceID]
	if !ok {
		backend.mu.Unlock()
		return ExecResult{}, fmt.Errorf("loopback: unknown instance %s", request.InstanceID)
	}
	if backend.destroyed != nil && containsString(backend.destroyed, request.InstanceID) {
		backend.mu.Unlock()
		return ExecResult{}, fmt.Errorf("loopback: instance %s destroyed", request.InstanceID)
	}
	backend.mu.Unlock()

	timeout := request.Timeout
	if timeout <= 0 {
		timeout = backend.timeout
	}
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	command := exec.CommandContext(runCtx, "sh", "-c", request.Command)
	command.Dir = workdir
	command.Env = sanitizedEnv(workdir)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output

	if err := command.Start(); err != nil {
		return ExecResult{}, err
	}
	// Publish the process under the mutex only after Start returns: any
	// concurrent Cancel/Pause/Resume/Destroy then sees it through the same
	// lock, establishing a happens-before edge on the *os.Process pointer.
	backend.mu.Lock()
	backend.procs[request.InstanceID] = command.Process
	backend.mu.Unlock()
	defer func() {
		backend.mu.Lock()
		delete(backend.procs, request.InstanceID)
		backend.mu.Unlock()
	}()

	runErr := command.Wait()
	exitCode := 0
	if runErr != nil {
		var exitErr *exec.ExitError
		if errors.As(runErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else if runCtx.Err() != nil {
			exitCode = 124 // timeout
		} else {
			return ExecResult{}, runErr
		}
	}
	digest := digestBytes(output.Bytes())
	return ExecResult{ExitCode: exitCode, OutputDigest: digest}, nil
}

func (backend *LoopbackBackend) Cancel(ctx context.Context, instanceID string) error {
	return backend.signal(instanceID, func(process *os.Process) error {
		if process == nil {
			return nil
		}
		return process.Kill()
	})
}

func (backend *LoopbackBackend) Pause(ctx context.Context, instanceID string) error {
	return backend.signal(instanceID, func(process *os.Process) error {
		if process == nil {
			return nil
		}
		return process.Signal(syscall.SIGSTOP)
	})
}

func (backend *LoopbackBackend) Resume(ctx context.Context, instanceID string) error {
	return backend.signal(instanceID, func(process *os.Process) error {
		if process == nil {
			return nil
		}
		return process.Signal(syscall.SIGCONT)
	})
}

func (backend *LoopbackBackend) Destroy(ctx context.Context, instanceID string) error {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	if process := backend.procs[instanceID]; process != nil {
		_ = process.Kill()
	}
	if workdir, ok := backend.dirs[instanceID]; ok {
		_ = os.RemoveAll(workdir)
		delete(backend.dirs, instanceID)
	}
	if !containsString(backend.destroyed, instanceID) {
		backend.destroyed = append(backend.destroyed, instanceID)
	}
	return nil
}

func (backend *LoopbackBackend) InspectCapabilities(ctx context.Context) (Capabilities, error) {
	return Capabilities{Supported: []string{
		"create", "exec", "cancel", "pause", "resume", "destroy",
	}}, nil
}

func (backend *LoopbackBackend) signal(instanceID string, op func(*os.Process) error) error {
	backend.mu.Lock()
	process := backend.procs[instanceID]
	backend.mu.Unlock()
	return op(process)
}

func sanitizedEnv(workdir string) []string {
	return []string{
		"PATH=/usr/bin:/bin:/usr/sbin:/sbin",
		"HOME=" + workdir,
		"TMPDIR=" + filepath.Join(workdir, "tmp"),
		"LANG=C",
		"LC_ALL=C",
	}
}

func digestBytes(data []byte) string {
	if len(data) > loopbackOutputLimit {
		data = data[len(data)-loopbackOutputLimit:]
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
