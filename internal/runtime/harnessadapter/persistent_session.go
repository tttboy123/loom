package harnessadapter

import (
	"context"
	"time"

	loomruntime "loom-pi-rebuild/internal/runtime"
)

type HarnessSessionRequest struct {
	ExecutablePath string
	Arguments      []string
	Environment    []string
	Directory      string
	MaxOutputBytes int
	Timeout        time.Duration
}

type HarnessStreamSession interface {
	WriteLine(context.Context, []byte) error
	ReadLine(context.Context) ([]byte, error)
	CloseInput() error
	Wait(context.Context) (HarnessCommandResult, error)
	Abort() error
}

type HarnessSessionRunner interface {
	StartSession(context.Context, HarnessSessionRequest) (HarnessStreamSession, error)
}

type HarnessAgentInputRunner interface {
	HarnessProcessRunner
	SupportsAgentInputs() bool
	RunHarnessWithAgentInputs(
		context.Context,
		HarnessProcessRequest,
		[]byte,
		loomruntime.AgentInputSource,
	) (HarnessProcessResult, error)
}
