package sandbox

import (
	"context"
	"sync"
	"testing"
	"time"
)

type recordingBackend struct {
	mu       sync.Mutex
	created  []CreateRequest
	execs    []ExecRequest
	cancels  []string
	pauses   []string
	resumes  []string
	destroys []string
}

func (backend *recordingBackend) Create(ctx context.Context, request CreateRequest) (Instance, error) {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	backend.created = append(backend.created, request)
	return Instance{InstanceID: "inst-" + request.JobID}, nil
}
func (backend *recordingBackend) Exec(ctx context.Context, request ExecRequest) (ExecResult, error) {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	backend.execs = append(backend.execs, request)
	return ExecResult{ExitCode: 0, OutputDigest: "sha256:out"}, nil
}
func (backend *recordingBackend) Cancel(ctx context.Context, instanceID string) error {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	backend.cancels = append(backend.cancels, instanceID)
	return nil
}
func (backend *recordingBackend) Pause(ctx context.Context, instanceID string) error {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	backend.pauses = append(backend.pauses, instanceID)
	return nil
}
func (backend *recordingBackend) Resume(ctx context.Context, instanceID string) error {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	backend.resumes = append(backend.resumes, instanceID)
	return nil
}
func (backend *recordingBackend) Destroy(ctx context.Context, instanceID string) error {
	backend.mu.Lock()
	defer backend.mu.Unlock()
	backend.destroys = append(backend.destroys, instanceID)
	return nil
}
func (backend *recordingBackend) InspectCapabilities(ctx context.Context) (Capabilities, error) {
	return Capabilities{Supported: []string{"exec", "cancel"}}, nil
}

func TestRedSB1_SevenOpsRecorded(t *testing.T) {
	backend := &recordingBackend{}
	ctx := context.Background()
	inst, err := backend.Create(ctx, CreateRequest{JobID: "j", RunID: "r", Generation: 1})
	if err != nil || inst.InstanceID == "" {
		t.Fatalf("create = %+v %v", inst, err)
	}
	if _, err := backend.Exec(ctx, ExecRequest{InstanceID: inst.InstanceID, Command: "go test ./...", Timeout: time.Second}); err != nil {
		t.Fatal(err)
	}
	_ = backend.Cancel(ctx, inst.InstanceID)
	_ = backend.Pause(ctx, inst.InstanceID)
	_ = backend.Resume(ctx, inst.InstanceID)
	_ = backend.Destroy(ctx, inst.InstanceID)
	if caps, err := backend.InspectCapabilities(ctx); err != nil || len(caps.Supported) != 2 {
		t.Fatalf("caps = %+v %v", caps, err)
	}
	if len(backend.created) != 1 || len(backend.execs) != 1 ||
		len(backend.cancels) != 1 || len(backend.pauses) != 1 ||
		len(backend.resumes) != 1 || len(backend.destroys) != 1 {
		t.Fatalf("ops not recorded: %+v", backend)
	}
}
