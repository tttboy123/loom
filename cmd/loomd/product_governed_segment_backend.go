package main

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/harnessgateway"
)

type productGovernedSegmentBackend struct {
	id        harnessgateway.BackendID
	version   int
	harnessID harnessgateway.HarnessID
	executor  api.LocalProductConversationResponder
}

func newProductGovernedSegmentBackend(
	harnessID harnessgateway.HarnessID,
	executor api.LocalProductConversationResponder,
) (*productGovernedSegmentBackend, error) {
	known := false
	for _, candidate := range harnessgateway.BuiltInHarnessIDs() {
		if candidate == harnessID {
			known = true
			break
		}
	}
	if !known || nilProductAssetPort(executor) {
		return nil, api.ErrLocalProductChatUnavailable
	}
	return &productGovernedSegmentBackend{
		id:        harnessgateway.BackendID("backend.segment." + string(harnessID)),
		version:   productHarnessGatewayBackendVersion,
		harnessID: harnessID,
		executor:  executor,
	}, nil
}

func (backend *productGovernedSegmentBackend) ID() harnessgateway.BackendID {
	if backend == nil {
		return ""
	}
	return backend.id
}

func (backend *productGovernedSegmentBackend) Version() int {
	if backend == nil {
		return 0
	}
	return backend.version
}

func (backend *productGovernedSegmentBackend) OpenSession(
	ctx context.Context,
	configured harnessgateway.ConfiguredHarness,
	binding harnessgateway.SegmentSessionBinding,
	workspace harnessgateway.Workspace,
) (harnessgateway.BackendSession, error) {
	if backend == nil || nilProductAssetPort(backend.executor) || ctx == nil || ctx.Err() != nil ||
		configured.HarnessID != backend.harnessID || configured.BackendID != backend.id ||
		configured.BackendVersion != backend.version ||
		binding.ConfiguredHarnessID != backend.harnessID || binding.BackendID != backend.id ||
		binding.BackendVersion != backend.version || workspace.ID != binding.WorkspaceID ||
		workspace.Digest != binding.WorkspaceDigest || !filepath.IsAbs(workspace.Path) ||
		filepath.Clean(workspace.Path) != workspace.Path {
		return nil, api.ErrLocalProductChatUnavailable
	}
	return &productGovernedSegmentSession{
		binding: binding, executor: backend.executor,
	}, nil
}

type productGovernedSegmentSession struct {
	binding  harnessgateway.SegmentSessionBinding
	executor api.LocalProductConversationResponder
	mu       sync.Mutex
	closed   bool
}

func (session *productGovernedSegmentSession) Respond(
	ctx context.Context,
	request harnessgateway.ResponseRequest,
) (harnessgateway.Response, error) {
	if session == nil || nilProductAssetPort(session.executor) || ctx == nil || ctx.Err() != nil {
		return harnessgateway.Response{}, api.ErrLocalProductChatUnavailable
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed {
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
	response, err := session.executor.Respond(ctx, decoded)
	if err != nil {
		return harnessgateway.Response{}, err
	}
	// This compatibility backend has no authenticated, turn-scoped control
	// channel. It must never accept model-asserted tool activity or proposals.
	if len(response.CompletedControlTools) != 0 ||
		len(response.ControlProposals) != 0 || len(response.ActionProposals) != 0 {
		return harnessgateway.Response{}, errors.Join(
			harnessgateway.ErrSessionUnhealthy,
			api.ErrLocalProductChatUnavailable,
		)
	}
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
	return harnessgateway.Response{Content: payload}, nil
}

func (session *productGovernedSegmentSession) Close(context.Context) error {
	if session == nil {
		return nil
	}
	session.mu.Lock()
	session.closed = true
	session.mu.Unlock()
	return nil
}
