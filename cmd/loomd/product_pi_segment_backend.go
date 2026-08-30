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
	"loom-pi-rebuild/internal/runtime/piadapter"
)

const (
	productPiSegmentBackendID       = harnessgateway.BackendID("backend.pi.rpc-control")
	productPiSegmentBackendVersion  = 1
	productPiConversationProviderID = "loom-local"
	productPiConversationModelID    = "qwen2.5-coder-1.5b-instruct-q4-k-m"
)

type productPiControlConversationResponder interface {
	respondWithControl(
		context.Context,
		api.LocalProductConversationRequest,
		piadapter.PiRPCConversationControlConfig,
	) (api.LocalProductConversationResponse, error)
}

type productPiSegmentBackendConfig struct {
	Responder       productPiControlConversationResponder
	ControlRegistry *controltool.Registry
	ControlGateway  controltool.Gateway
}

type productPiSegmentBackend struct {
	config productPiSegmentBackendConfig
}

func newProductPiSegmentBackend(
	config productPiSegmentBackendConfig,
) (*productPiSegmentBackend, error) {
	if nilProductAssetPort(config.Responder) || config.ControlRegistry == nil ||
		nilProductAssetPort(config.ControlGateway) {
		return nil, api.ErrLocalProductChatUnavailable
	}
	return &productPiSegmentBackend{config: config}, nil
}

func (*productPiSegmentBackend) ID() harnessgateway.BackendID {
	return productPiSegmentBackendID
}

func (*productPiSegmentBackend) Version() int {
	return productPiSegmentBackendVersion
}

func (backend *productPiSegmentBackend) OpenSession(
	ctx context.Context,
	configured harnessgateway.ConfiguredHarness,
	binding harnessgateway.SegmentSessionBinding,
	workspace harnessgateway.Workspace,
) (harnessgateway.BackendSession, error) {
	if backend == nil || ctx == nil || ctx.Err() != nil ||
		configured.HarnessID != harnessgateway.HarnessPi ||
		configured.BackendID != backend.ID() || configured.BackendVersion != backend.Version() ||
		binding.ConfiguredHarnessID != harnessgateway.HarnessPi ||
		binding.BackendID != backend.ID() || binding.BackendVersion != backend.Version() ||
		binding.ProviderID != productPiConversationProviderID ||
		binding.ProviderAccountID != "" || binding.CredentialRevision != 0 ||
		binding.ModelID != productPiConversationModelID ||
		workspace.ID != binding.WorkspaceID || workspace.Digest != binding.WorkspaceDigest ||
		!filepath.IsAbs(workspace.Path) || filepath.Clean(workspace.Path) != workspace.Path ||
		binding.SessionID() == "" {
		return nil, api.ErrLocalProductChatUnavailable
	}
	controlLifetime, cancelControl := context.WithCancel(context.Background())
	controlServer, err := harnessadapter.OpenHarnessControlServer(
		controlLifetime, backend.config.ControlRegistry, backend.config.ControlGateway,
	)
	if err != nil {
		cancelControl()
		return nil, errors.Join(api.ErrLocalProductChatUnavailable, err)
	}
	return &productPiSegmentSession{
		binding: binding, config: backend.config,
		controlServer: controlServer, cancelControl: cancelControl,
	}, nil
}

type productPiSegmentSession struct {
	binding       harnessgateway.SegmentSessionBinding
	config        productPiSegmentBackendConfig
	controlServer *harnessadapter.HarnessControlServer
	cancelControl context.CancelFunc
	mu            sync.Mutex
	closed        bool
}

func (session *productPiSegmentSession) Respond(
	ctx context.Context,
	request harnessgateway.ResponseRequest,
) (harnessgateway.Response, error) {
	if session == nil || ctx == nil || ctx.Err() != nil {
		return harnessgateway.Response{}, api.ErrLocalProductChatUnavailable
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed || session.controlServer == nil ||
		nilProductAssetPort(session.config.Responder) {
		return harnessgateway.Response{}, errors.Join(
			harnessgateway.ErrSessionUnhealthy, api.ErrLocalProductChatUnavailable,
		)
	}
	var decoded api.LocalProductConversationRequest
	if json.Unmarshal(request.Input, &decoded) != nil ||
		!productHarnessGatewayRequestMatches(decoded, request.Authority, session.binding) ||
		decoded.ExecutionBinding == nil ||
		decoded.ExecutionBinding.HarnessAdapter != "pi" ||
		decoded.ExecutionBinding.ProviderID != productPiConversationProviderID ||
		decoded.ExecutionBinding.ProviderAccountID != "" ||
		decoded.ExecutionBinding.CredentialRevision != 0 ||
		decoded.ExecutionBinding.ModelID != productPiConversationModelID ||
		decoded.ModelID != productPiConversationModelID {
		return harnessgateway.Response{}, api.ErrLocalProductChatUnavailable
	}
	if err := session.controlServer.BeginTurn(
		productHarnessControlTurnContext(decoded, session.binding),
	); err != nil {
		return harnessgateway.Response{}, errors.Join(
			harnessgateway.ErrSessionUnhealthy, api.ErrLocalProductChatUnavailable,
		)
	}
	control, controlErr := productPiConversationControlConfig(
		session.config.ControlRegistry, session.controlServer.Lease(),
	)
	if controlErr != nil {
		_, _ = session.controlServer.EndTurn()
		return harnessgateway.Response{}, errors.Join(
			harnessgateway.ErrSessionUnhealthy, api.ErrLocalProductChatUnavailable,
		)
	}
	response, respondErr := session.config.Responder.respondWithControl(
		ctx, decoded, control,
	)
	proposals, endErr := session.controlServer.EndTurn()
	if respondErr != nil {
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
			harnessgateway.ErrSessionUnhealthy, api.ErrLocalProductChatUnavailable,
		)
	}
	payload, err := json.Marshal(response)
	if err != nil || len(payload) > productHarnessGatewayMaximumOutput {
		clearProductHarnessGatewayBytes(payload)
		return harnessgateway.Response{}, api.ErrLocalProductChatUnavailable
	}
	return harnessgateway.Response{Content: payload}, nil
}

func productPiConversationControlConfig(
	registry *controltool.Registry,
	lease harnessadapter.HarnessControlMCPLease,
) (piadapter.PiRPCConversationControlConfig, error) {
	if registry == nil || lease.URL == "" || lease.Token == "" {
		return piadapter.PiRPCConversationControlConfig{}, api.ErrLocalProductChatUnavailable
	}
	definitions := registry.Definitions()
	if len(definitions) == 0 || len(definitions) != len(lease.ToolNames) {
		return piadapter.PiRPCConversationControlConfig{}, api.ErrLocalProductChatUnavailable
	}
	control := piadapter.PiRPCConversationControlConfig{
		URL: lease.URL, Token: lease.Token,
		Tools: make([]piadapter.PiRPCConversationControlTool, len(definitions)),
	}
	for index, definition := range definitions {
		if definition.MCPName != lease.ToolNames[index] {
			return piadapter.PiRPCConversationControlConfig{}, api.ErrLocalProductChatUnavailable
		}
		control.Tools[index] = piadapter.PiRPCConversationControlTool{
			Name: definition.MCPName, Description: definition.Description,
			InputSchema: append(json.RawMessage(nil), definition.InputSchema...),
		}
	}
	return control, nil
}

func (session *productPiSegmentSession) Close(ctx context.Context) error {
	if session == nil || ctx == nil {
		return api.ErrLocalProductChatUnavailable
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed {
		return nil
	}
	session.closed = true
	if session.controlServer != nil {
		session.controlServer.Close()
		session.controlServer = nil
	}
	if session.cancelControl != nil {
		session.cancelControl()
		session.cancelControl = nil
	}
	return nil
}
