// Package sandbox defines the vendor-neutral governed sandbox capability
// contract (Phase 3B). The backend is never a state authority: Loom rebuilds
// and reconciles from the Event Journal. One backend per implementation.
package sandbox

import (
	"context"
	"time"
)

type CreateRequest struct {
	JobID           string
	RunID           string
	Generation      int64
	WorkspaceDigest string
}

type Instance struct {
	InstanceID string
}

type ExecRequest struct {
	InstanceID string
	Command    string
	Timeout    time.Duration
	EnvDigest  string
}

type ExecResult struct {
	ExitCode     int
	OutputDigest string
}

type Capabilities struct {
	Supported []string
}

// SandboxBackend is the frozen 7-capability interface.
type SandboxBackend interface {
	Create(context.Context, CreateRequest) (Instance, error)
	Exec(context.Context, ExecRequest) (ExecResult, error)
	Cancel(context.Context, string) error
	Pause(context.Context, string) error
	Resume(context.Context, string) error
	Destroy(context.Context, string) error
	InspectCapabilities(context.Context) (Capabilities, error)
}
