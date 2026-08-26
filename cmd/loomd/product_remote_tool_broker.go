package main

import (
	"context"
	"errors"
	"sync"
	"time"

	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/permissions"
	"loom-pi-rebuild/internal/toolbroker"
)

type productRemoteToolBrokerConfig struct {
	Search         toolbroker.SearchBackend
	WebFetch       bool
	MCPClients     map[string]toolbroker.MCPToolClient
	MCPAllowlist   map[string][]string
	Timeout        time.Duration
	MaxResultBytes int
	// EnrollmentOnly keeps the transport available for persisted, governed
	// Enrollment materialization without exposing an unenrolled global tool.
	EnrollmentOnly bool
}

// newProductDefaultRemoteToolBrokerConfig returns an explicitly-opted
// remote-tool configuration: a real governed web-search backend
// (DuckDuckGo Lite over the SSRF-safe transport) plus bounded WebFetch.
// Production composition stays fail-closed (nil) unless a caller opts in;
// web_search enrollments only materialize when a trusted Search backend is
// injected.
func newProductDefaultRemoteToolBrokerConfig() (*productRemoteToolBrokerConfig, error) {
	search, err := toolbroker.NewDDGSearchClient(20*time.Second, 5, 48<<10)
	if err != nil {
		return nil, err
	}
	return &productRemoteToolBrokerConfig{
		Search: search, WebFetch: true,
		Timeout:        20 * time.Second,
		MaxResultBytes: 32 << 10,
	}, nil
}

type productRemoteToolExecutor struct {
	mu        sync.RWMutex
	broker    *toolbroker.Broker
	lifecycle context.Context
	cancel    context.CancelCauseFunc
	close     func()
	closed    bool
}

func newProductRemoteToolBroker(
	ctx context.Context,
	config *productRemoteToolBrokerConfig,
) (execution.RemoteToolExecutor, composition.Effect, error) {
	if ctx == nil {
		return nil, nil, toolbroker.ErrInvalidConfig
	}
	if config == nil || config.EnrollmentOnly {
		return nil, nil, nil
	}
	if len(config.MCPClients) != len(config.MCPAllowlist) {
		return nil, nil, toolbroker.ErrInvalidConfig
	}
	for server := range config.MCPClients {
		if len(config.MCPAllowlist[server]) == 0 {
			return nil, nil, toolbroker.ErrInvalidConfig
		}
	}
	for server := range config.MCPAllowlist {
		if _, exists := config.MCPClients[server]; !exists {
			return nil, nil, toolbroker.ErrInvalidConfig
		}
	}

	var (
		httpClientClose func()
		httpClient      toolbroker.HTTPDoer
	)
	if config.WebFetch {
		client, err := toolbroker.NewSystemHTTPClient(config.Timeout)
		if err != nil {
			return nil, nil, err
		}
		httpClient = client
		httpClientClose = client.CloseIdleConnections
	}

	var mcpRegistry toolbroker.MCPCaller
	if len(config.MCPClients) > 0 {
		registry, err := toolbroker.NewMCPRegistry(config.MCPClients)
		if err != nil {
			if httpClientClose != nil {
				httpClientClose()
			}
			return nil, nil, err
		}
		mcpRegistry = registry
	}
	broker, err := toolbroker.New(toolbroker.Config{
		Search: config.Search, HTTP: httpClient, MCP: mcpRegistry,
		MCPAllowlist: config.MCPAllowlist, Timeout: config.Timeout,
		MaxResultBytes: config.MaxResultBytes,
	})
	if err != nil {
		if httpClientClose != nil {
			httpClientClose()
		}
		return nil, nil, err
	}
	lifecycle, cancel := context.WithCancelCause(context.Background())
	executor := &productRemoteToolExecutor{
		broker: broker, lifecycle: lifecycle, cancel: cancel,
		close: httpClientClose,
	}
	effect := composition.NewEffect(func(context.Context) error {
		executor.mu.Lock()
		if executor.closed {
			executor.mu.Unlock()
			return nil
		}
		executor.closed = true
		executor.cancel(toolbroker.ErrToolDenied)
		if executor.close != nil {
			executor.close()
		}
		executor.broker = nil
		executor.mu.Unlock()
		return nil
	})
	if effect == nil {
		return nil, nil, toolbroker.ErrInvalidConfig
	}
	return executor, effect, nil
}

func (executor *productRemoteToolExecutor) AllowedRemoteTools() []permissions.ToolKind {
	if executor == nil {
		return nil
	}
	executor.mu.RLock()
	defer executor.mu.RUnlock()
	if executor.closed || executor.broker == nil {
		return nil
	}
	return executor.broker.AllowedRemoteTools()
}

func (executor *productRemoteToolExecutor) ValidateProposal(
	proposal permissions.ProposedCall,
) error {
	if executor == nil {
		return toolbroker.ErrToolDenied
	}
	executor.mu.RLock()
	if executor.closed || executor.broker == nil {
		executor.mu.RUnlock()
		return toolbroker.ErrToolDenied
	}
	broker := executor.broker
	executor.mu.RUnlock()
	return broker.ValidateProposal(proposal)
}

func (executor *productRemoteToolExecutor) ExecuteProposalContent(
	ctx context.Context,
	proposal permissions.ProposedCall,
) ([]byte, error) {
	if executor == nil || ctx == nil {
		return nil, toolbroker.ErrToolDenied
	}
	executor.mu.RLock()
	if executor.closed || executor.broker == nil {
		executor.mu.RUnlock()
		return nil, toolbroker.ErrToolDenied
	}
	broker := executor.broker
	lifecycle := executor.lifecycle
	executor.mu.RUnlock()
	callContext, cancel := context.WithCancelCause(ctx)
	stop := context.AfterFunc(lifecycle, func() {
		cancel(context.Cause(lifecycle))
	})
	defer func() {
		stop()
		cancel(nil)
	}()
	content, err := broker.ExecuteProposalContent(callContext, proposal)
	if err != nil {
		return nil, err
	}
	executor.mu.RLock()
	closed := executor.closed
	executor.mu.RUnlock()
	if cause := context.Cause(lifecycle); cause != nil || closed {
		for index := range content {
			content[index] = 0
		}
		if cause == nil {
			cause = toolbroker.ErrToolDenied
		}
		return nil, errors.Join(toolbroker.ErrToolFailed, cause)
	}
	return content, nil
}
