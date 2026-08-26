package main

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/harnessgateway"
	"loom-pi-rebuild/internal/runtime/harnessadapter"
)

const (
	productClaudeCodeSegmentBackendID = harnessgateway.BackendID(
		"backend.claude-code.print-session",
	)
	productClaudeCodeSegmentBackendVersion = 2
)

type productClaudeCodeSegmentBackendConfig struct {
	Responder *productClaudeCodeConversationResponder
}

type productClaudeCodeSegmentBackend struct {
	responder *productClaudeCodeConversationResponder
}

func newProductClaudeCodeSegmentBackend(
	config productClaudeCodeSegmentBackendConfig,
) (*productClaudeCodeSegmentBackend, error) {
	if config.Responder == nil {
		return nil, api.ErrLocalProductChatUnavailable
	}
	return &productClaudeCodeSegmentBackend{responder: config.Responder}, nil
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
	return &productClaudeCodeSegmentSession{
		binding:   binding,
		responder: backend.responder,
		nativeSessionID: productDeterministicUUID(
			"harness-gateway", "claude-code", sessionID,
		),
	}, nil
}

type productClaudeCodeSegmentSession struct {
	binding         harnessgateway.SegmentSessionBinding
	responder       *productClaudeCodeConversationResponder
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
	response, err := session.responder.respondWithNativeSession(
		ctx, decoded, session.nativeSessionID, session.turns > 0,
	)
	if err != nil {
		if errors.Is(err, harnessadapter.ErrHarnessProtocol) {
			err = errors.Join(harnessgateway.ErrSessionUnhealthy, err)
		}
		return harnessgateway.Response{}, err
	}
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
	session.mu.Unlock()
	return nil
}
