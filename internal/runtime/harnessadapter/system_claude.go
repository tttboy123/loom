package harnessadapter

import (
	"crypto/rand"
	"net/http"
	"time"

	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/runtime/nativeadapter"
	"loom-pi-rebuild/internal/supervisor"
)

func NewSystemClaudeCodeAgentAdapter(
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
		return nil, ErrInvalidClaudeCodeAdapter
	}
	resolved, err := ResolveHarnessExecutable(executablePath)
	if err != nil || resolved != executablePath {
		return nil, ErrInvalidClaudeCodeAdapter
	}
	transport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, ErrInvalidClaudeCodeAdapter
	}
	privateTransport := transport.Clone()
	privateTransport.Proxy = nil
	privateTransport.DisableCompression = true
	gateway, err := NewAnthropicAttemptGateway(AttemptGatewayConfig{
		Client: &http.Client{
			Transport: privateTransport,
			Timeout:   providerTimeout,
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		Random:           rand.Reader,
		MaxRequestBytes:  4 << 20,
		MaxResponseBytes: 16 << 20,
		MaxRequests:      128,
	})
	if err != nil {
		return nil, err
	}
	runner, err := NewClaudeCodeProcessRunner(ClaudeCodeProcessRunnerConfig{
		Gateway:  gateway,
		Commands: NewSystemHarnessCommandRunner(),
		Sessions: NewSystemHarnessSessionRunner(),
	})
	if err != nil {
		return nil, err
	}
	var toolGateway loomruntime.AttemptToolGateway
	if len(toolGateways) == 1 && toolGateways[0] != nil {
		toolGateway = toolGateways[0]
	}
	return NewClaudeCodeAdapter(ClaudeCodeAdapterConfig{
		RuntimeInstanceID: runtimeInstanceID,
		ExecutablePath:    executablePath,
		CredentialAccess:  credentialAccess,
		Diagnostics:       diagnostics,
		Runner:            runner,
		Now:               now,
		MaxOutputBytes:    maxOutputBytes,
		ToolGateway:       toolGateway,
	})
}
