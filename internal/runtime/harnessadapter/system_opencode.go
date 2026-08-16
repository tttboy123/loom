package harnessadapter

import (
	"time"

	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/nativeadapter"
	"loom-pi-rebuild/internal/supervisor"
)

// NewSystemOpenCodeAgentAdapter builds the OpenCode Harness adapter for the
// installed OpenCode CLI. OpenCode keeps its own native auth; brokered Loom
// credentials are injected per binding through the provider's well-known
// environment variable by the process runner.
func NewSystemOpenCodeAgentAdapter(
	runtimeInstanceID string,
	executablePath string,
	credentialAccess nativeadapter.CredentialAccess,
	diagnostics nativeadapter.AgentAttemptDiagnosticRecorder,
	now func() time.Time,
	providerTimeout time.Duration,
	maxOutputBytes int,
	toolGateways ...loomruntime.AttemptToolGateway,
) (supervisor.RuntimeAdapter, error) {
	if providerTimeout <= 0 || providerTimeout > 15*time.Minute || len(toolGateways) > 1 ||
		len(toolGateways) == 1 && toolGateways[0] != nil && nilHarnessInterface(toolGateways[0]) {
		return nil, ErrInvalidOpenCodeAdapter
	}
	resolved, err := ResolveHarnessExecutable(executablePath)
	if err != nil || resolved != executablePath {
		return nil, ErrInvalidOpenCodeAdapter
	}
	runner, err := NewOpenCodeProcessRunner(OpenCodeProcessRunnerConfig{
		Commands: NewSystemHarnessCommandRunner(),
	})
	if err != nil {
		return nil, err
	}
	var toolGateway loomruntime.AttemptToolGateway
	if len(toolGateways) == 1 && toolGateways[0] != nil {
		toolGateway = toolGateways[0]
	}
	return NewOpenCodeAdapter(OpenCodeAdapterConfig{
		RuntimeInstanceID: runtimeInstanceID, ExecutablePath: executablePath,
		CredentialAccess: credentialAccess, Diagnostics: diagnostics,
		Runner: runner, Now: now, MaxOutputBytes: maxOutputBytes,
		ToolGateway: toolGateway,
	})
}
