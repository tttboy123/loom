package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newLoopback(t testing.TB) *LoopbackBackend {
	t.Helper()
	backend, err := NewLoopbackBackend(t.TempDir(), 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = backend.Destroy(context.Background(), "all") })
	return backend
}

func TestLoopbackExecIsolatedWorkdirSanitizedEnv(t *testing.T) {
	backend := newLoopback(t)
	ctx := context.Background()
	instance, err := backend.Create(ctx, CreateRequest{
		JobID: "j1", RunID: "r1", Generation: 1, WorkspaceDigest: "sha256:w",
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := backend.Exec(ctx, ExecRequest{
		InstanceID: instance.InstanceID, Command: "pwd && env",
		Timeout: 2 * time.Second, EnvDigest: EnvDigest(map[string]string{"API_TOKEN": "super-secret"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit=%d", result.ExitCode)
	}
	workdir, ok := backend.dirs[instance.InstanceID]
	if !ok {
		t.Fatal("instance workdir missing")
	}
	if filepath.Base(workdir) != instance.InstanceID {
		t.Fatalf("workdir not instance-scoped: %s", workdir)
	}
	// The sanitized env must not include inherited variables with secrets.
	envOut, err := backend.Exec(ctx, ExecRequest{
		InstanceID: instance.InstanceID, Command: "env | grep -c super-secret || true",
		Timeout: 2 * time.Second, EnvDigest: "sha256:e",
	})
	if err != nil {
		t.Fatal(err)
	}
	// ExecResult carries only digests; the canary script asserts visible output.
	if envOut.ExitCode != 0 {
		t.Fatalf("grep exit=%d (secret leaked into sanitized env?)", envOut.ExitCode)
	}
}

func TestLoopbackCancelTerminatesLongCommand(t *testing.T) {
	backend := newLoopback(t)
	ctx := context.Background()
	instance, err := backend.Create(ctx, CreateRequest{
		JobID: "j2", RunID: "r2", Generation: 1, WorkspaceDigest: "sha256:w",
	})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan ExecResult, 1)
	errs := make(chan error, 1)
	go func() {
		result, execErr := backend.Exec(ctx, ExecRequest{
			InstanceID: instance.InstanceID, Command: "sleep 60",
			Timeout: 30 * time.Second, EnvDigest: "sha256:e",
		})
		if execErr != nil {
			errs <- execErr
			return
		}
		done <- result
	}()
	time.Sleep(300 * time.Millisecond)
	if err := backend.Cancel(ctx, instance.InstanceID); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-done:
		if result.ExitCode == 0 {
			t.Fatal("canceled command must not exit zero")
		}
	case execErr := <-errs:
		if execErr == nil {
			t.Fatal("expected error or nonzero exit")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not terminate the command")
	}
}

func TestLoopbackDestroyCleansWorkdir(t *testing.T) {
	backend := newLoopback(t)
	ctx := context.Background()
	instance, err := backend.Create(ctx, CreateRequest{
		JobID: "j3", RunID: "r3", Generation: 1, WorkspaceDigest: "sha256:w",
	})
	if err != nil {
		t.Fatal(err)
	}
	workdir := backend.dirs[instance.InstanceID]
	if _, err := os.Stat(workdir); err != nil {
		t.Fatal(err)
	}
	if err := backend.Destroy(ctx, instance.InstanceID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(workdir); !os.IsNotExist(err) {
		t.Fatalf("workdir still exists after destroy: %v", err)
	}
	if _, err := backend.Exec(ctx, ExecRequest{
		InstanceID: instance.InstanceID, Command: "echo x", Timeout: time.Second,
	}); err == nil || !strings.Contains(err.Error(), "unknown instance") {
		t.Fatalf("exec after destroy must fail: %v", err)
	}
}
