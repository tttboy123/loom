package main

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/controltool"
	"loom-pi-rebuild/internal/harnessgateway"
	"loom-pi-rebuild/internal/runtime/harnessadapter"
)

const (
	productClaudeCodeSegmentBackendID = harnessgateway.BackendID(
		"backend.claude-code.print-session",
	)
	productClaudeCodeSegmentBackendVersion = 3
)

type productClaudeCodeSegmentBackendConfig struct {
	Responder       *productClaudeCodeConversationResponder
	ControlRegistry *controltool.Registry
	ControlGateway  controltool.Gateway
}

type productClaudeCodeSegmentBackend struct {
	responder       *productClaudeCodeConversationResponder
	controlRegistry *controltool.Registry
	controlGateway  controltool.Gateway
}

func newProductClaudeCodeSegmentBackend(
	config productClaudeCodeSegmentBackendConfig,
) (*productClaudeCodeSegmentBackend, error) {
	if config.Responder == nil ||
		(config.ControlRegistry == nil) != nilProductAssetPort(config.ControlGateway) {
		return nil, api.ErrLocalProductChatUnavailable
	}
	return &productClaudeCodeSegmentBackend{
		responder: config.Responder, controlRegistry: config.ControlRegistry,
		controlGateway: config.ControlGateway,
	}, nil
}

func (*productClaudeCodeSegmentBackend) ID() harnessgateway.BackendID {
	return productClaudeCodeSegmentBackendID
}

func (*productClaudeCodeSegmentBackend) Version() int {
	return productClaudeCodeSegmentBackendVersion
}

func (backend *productClaudeCodeSegmentBackend) OpenSession(
	ctx context.Context,
	configured harnessgateway.ConfiguredHarness,
	binding harnessgateway.SegmentSessionBinding,
	workspace harnessgateway.Workspace,
) (harnessgateway.BackendSession, error) {
	if backend == nil || backend.responder == nil || ctx == nil || ctx.Err() != nil ||
		configured.HarnessID != harnessgateway.HarnessClaudeCode ||
		configured.BackendID != backend.ID() || configured.BackendVersion != backend.Version() ||
		binding.ConfiguredHarnessID != harnessgateway.HarnessClaudeCode ||
		binding.BackendID != backend.ID() || binding.BackendVersion != backend.Version() ||
		workspace.ID != binding.WorkspaceID || workspace.Digest != binding.WorkspaceDigest ||
		!filepath.IsAbs(workspace.Path) || filepath.Clean(workspace.Path) != workspace.Path {
		return nil, api.ErrLocalProductChatUnavailable
	}
	sessionID := binding.SessionID()
	if sessionID == "" {
		return nil, api.ErrLocalProductChatUnavailable
	}
	systemPrompt, err := productConversationControlSystemPrompt(
		binding.ProviderID, binding.ModelID, "claude-code", backend.controlRegistry,
	)
	if err != nil {
		return nil, errors.Join(api.ErrLocalProductChatUnavailable, err)
	}
	var controlServer *harnessadapter.HarnessControlServer
	var cancelControl context.CancelFunc
	if backend.controlRegistry != nil {
		controlLifetime, cancel := context.WithCancel(context.Background())
		var err error
		controlServer, err = harnessadapter.OpenHarnessControlServer(
			controlLifetime, backend.controlRegistry, backend.controlGateway,
		)
		if err != nil {
			cancel()
			return nil, errors.Join(api.ErrLocalProductChatUnavailable, err)
		}
		cancelControl = cancel
	}
	return &productClaudeCodeSegmentSession{
		binding: binding, responder: backend.responder,
		controlServer: controlServer, cancelControl: cancelControl,
		systemPrompt: systemPrompt,
		nativeSessionID: productDeterministicUUID(
			"harness-gateway", "claude-code", sessionID,
		),
	}, nil
}

type productClaudeCodeSegmentSession struct {
	binding         harnessgateway.SegmentSessionBinding
	responder       *productClaudeCodeConversationResponder
	controlServer   *harnessadapter.HarnessControlServer
	cancelControl   context.CancelFunc
	systemPrompt    string
	nativeSessionID string
	mu              sync.Mutex
	turns           int
	closed          bool
}

func (session *productClaudeCodeSegmentSession) Respond(
	ctx context.Context,
	request harnessgateway.ResponseRequest,
) (harnessgateway.Response, error) {
	if session == nil || ctx == nil || ctx.Err() != nil {
		return harnessgateway.Response{}, api.ErrLocalProductChatUnavailable
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed || session.responder == nil {
		return harnessgateway.Response{}, errors.Join(
			harnessgateway.ErrSessionUnhealthy,
			api.ErrLocalProductChatUnavailable,
		)
	}
	var decoded api.LocalProductConversationRequest
	if json.Unmarshal(request.Input, &decoded) != nil ||
		!productHarnessGatewayRequestMatches(decoded, request.Authority, session.binding) {
		return harnessgateway.Response{}, api.ErrLocalProductChatUnavailable
	}
	controlLease := harnessadapter.HarnessControlMCPLease{}
	if session.controlServer != nil {
		if err := session.controlServer.BeginTurn(
			productHarnessControlTurnContext(decoded, session.binding),
		); err != nil {
			return harnessgateway.Response{}, errors.Join(
				harnessgateway.ErrSessionUnhealthy, api.ErrLocalProductChatUnavailable,
			)
		}
		controlLease = session.controlServer.Lease()
	}
	response, respondErr := session.responder.respondWithNativeSession(
		ctx, decoded, session.nativeSessionID, session.turns > 0, controlLease,
		session.systemPrompt,
	)
	var proposals controltool.ProposalBatch
	var endErr error
	if session.controlServer != nil {
		proposals, endErr = session.controlServer.EndTurn()
	}
	if respondErr != nil {
		if errors.Is(respondErr, harnessadapter.ErrHarnessProtocol) {
			respondErr = errors.Join(harnessgateway.ErrSessionUnhealthy, respondErr)
		}
		return harnessgateway.Response{}, respondErr
	}
	if endErr != nil {
		return harnessgateway.Response{}, errors.Join(
			harnessgateway.ErrSessionUnhealthy, api.ErrLocalProductChatUnavailable,
		)
	}
	response.ControlProposals = proposals.SessionAlignments
	response.ActionProposals = proposals.ConversationActions
	response.CompletedControlTools = proposals.CompletedCalls
	response = productHarnessGatewayResponseWithProposalFallback(response)
	if !validProductHarnessGatewayResponse(response) {
		return harnessgateway.Response{}, errors.Join(
			harnessgateway.ErrSessionUnhealthy,
			api.ErrLocalProductChatUnavailable,
		)
	}
	payload, err := json.Marshal(response)
	if err != nil || len(payload) > productHarnessGatewayMaximumOutput {
		clearProductHarnessGatewayBytes(payload)
		return harnessgateway.Response{}, errors.Join(
			harnessgateway.ErrSessionUnhealthy,
			api.ErrLocalProductChatUnavailable,
		)
	}
	session.turns++
	return harnessgateway.Response{Content: payload}, nil
}

func (session *productClaudeCodeSegmentSession) Close(context.Context) error {
	if session == nil {
		return nil
	}
	session.mu.Lock()
	session.closed = true
	if session.controlServer != nil {
		session.controlServer.Close()
		session.controlServer = nil
	}
	if session.cancelControl != nil {
		session.cancelControl()
		session.cancelControl = nil
	}
	session.mu.Unlock()
	return nil
}
