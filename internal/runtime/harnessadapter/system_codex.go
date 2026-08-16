package harnessadapter

import (
	"crypto/rand"
	"net/http"
	"time"

	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/nativeadapter"
	"loom-pi-rebuild/internal/supervisor"
)

func NewSystemCodexAgentAdapter(
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
		return nil, ErrInvalidCodexAdapter
	}
	resolved, err := ResolveHarnessExecutable(executablePath)
	if err != nil || resolved != executablePath {
		return nil, ErrInvalidCodexAdapter
	}
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, ErrInvalidCodexAdapter
	}
	privateTransport := transport.Clone()
	privateTransport.Proxy = nil
	privateTransport.DisableCompression = true
	gateway, err := NewOpenAIAttemptGateway(AttemptGatewayConfig{
		Client: &http.Client{
			Transport: privateTransport, Timeout: providerTimeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		Random: rand.Reader, MaxRequestBytes: 8 << 20,
		MaxResponseBytes: 32 << 20, MaxRequests: 256,
	})
	if err != nil {
		return nil, err
	}
	runner, err := NewCodexProcessRunner(CodexProcessRunnerConfig{
		Gateway: gateway, Commands: NewSystemHarnessCommandRunner(),
		Sessions: NewSystemHarnessSessionRunner(),
	})
	if err != nil {
		return nil, err
	}
	var toolGateway loomruntime.AttemptToolGateway
	if len(toolGateways) == 1 && toolGateways[0] != nil {
		toolGateway = toolGateways[0]
	}
	return NewCodexAdapter(CodexAdapterConfig{
		RuntimeInstanceID: runtimeInstanceID, ExecutablePath: executablePath,
		CredentialAccess: credentialAccess, Diagnostics: diagnostics,
		Runner: runner, Now: now, MaxOutputBytes: maxOutputBytes,
		ToolGateway: toolGateway,
	})
}
