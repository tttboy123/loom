package main

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"sync"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/controltool"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/harnessgateway"
	"loom-pi-rebuild/internal/provider"
	"loom-pi-rebuild/internal/runtime/harnessadapter"
)

const (
	productLoomNativeSegmentBackendID = harnessgateway.BackendID(
		"backend.loom-native.provider-control",
	)
	productLoomNativeSegmentBackendVersion = 1
)

type productLoomNativeControlConversationResponder interface {
	respondBrokeredWithControl(
		context.Context,
		api.LocalProductConversationRequest,
		[]provider.ConversationControlTool,
		provider.ConversationControlExecutor,
	) (api.LocalProductConversationResponse, error)
}

type productLoomNativeSegmentBackendConfig struct {
	Responder       productLoomNativeControlConversationResponder
	ControlRegistry *controltool.Registry
	ControlGateway  controltool.Gateway
}

type productLoomNativeSegmentBackend struct {
	config productLoomNativeSegmentBackendConfig
}

func newProductLoomNativeSegmentBackend(
	config productLoomNativeSegmentBackendConfig,
) (*productLoomNativeSegmentBackend, error) {
	if nilProductAssetPort(config.Responder) || config.ControlRegistry == nil ||
		nilProductAssetPort(config.ControlGateway) {
		return nil, api.ErrLocalProductChatUnavailable
	}
	return &productLoomNativeSegmentBackend{config: config}, nil
}

func (*productLoomNativeSegmentBackend) ID() harnessgateway.BackendID {
	return productLoomNativeSegmentBackendID
}

func (*productLoomNativeSegmentBackend) Version() int {
	return productLoomNativeSegmentBackendVersion
}

func (backend *productLoomNativeSegmentBackend) OpenSession(
	ctx context.Context,
	configured harnessgateway.ConfiguredHarness,
	binding harnessgateway.SegmentSessionBinding,
	workspace harnessgateway.Workspace,
) (harnessgateway.BackendSession, error) {
	if backend == nil || ctx == nil || ctx.Err() != nil ||
		configured.HarnessID != harnessgateway.HarnessLoomNative ||
		configured.BackendID != backend.ID() || configured.BackendVersion != backend.Version() ||
		binding.ConfiguredHarnessID != harnessgateway.HarnessLoomNative ||
		binding.BackendID != backend.ID() || binding.BackendVersion != backend.Version() ||
		binding.ProviderID == "" || binding.ProviderAccountID == "" ||
		!credentials.ValidProviderAccountIdentifier(
			binding.ProviderID, binding.ProviderAccountID,
		) || binding.CredentialRevision <= 0 || strings.TrimSpace(binding.ModelID) == "" ||
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
	return &productLoomNativeSegmentSession{
		binding: binding, config: backend.config,
		controlServer: controlServer, cancelControl: cancelControl,
	}, nil
}

type productLoomNativeSegmentSession struct {
	binding       harnessgateway.SegmentSessionBinding
	config        productLoomNativeSegmentBackendConfig
	controlServer *harnessadapter.HarnessControlServer
	cancelControl context.CancelFunc
	mu            sync.Mutex
	closed        bool
}

func (session *productLoomNativeSegmentSession) Respond(
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
		decoded.ExecutionBinding.HarnessAdapter != "loom-native" ||
		decoded.ExecutionBinding.ProviderID != session.binding.ProviderID ||
		decoded.ExecutionBinding.ProviderAccountID != session.binding.ProviderAccountID ||
		decoded.ExecutionBinding.CredentialRevision != session.binding.CredentialRevision ||
		decoded.ExecutionBinding.ModelID != session.binding.ModelID ||
		decoded.ModelID != session.binding.ModelID {
		return harnessgateway.Response{}, api.ErrLocalProductChatUnavailable
	}
	if err := session.controlServer.BeginTurn(
		productHarnessControlTurnContext(decoded, session.binding),
	); err != nil {
		return harnessgateway.Response{}, errors.Join(
			harnessgateway.ErrSessionUnhealthy, api.ErrLocalProductChatUnavailable,
		)
	}
	lease := session.controlServer.Lease()
	tools, toolsErr := productProviderConversationControlTools(
		session.config.ControlRegistry, lease,
	)
	if toolsErr != nil {
		_, _ = session.controlServer.EndTurn()
		return harnessgateway.Response{}, errors.Join(
			harnessgateway.ErrSessionUnhealthy, api.ErrLocalProductChatUnavailable,
		)
	}
	response, respondErr := session.config.Responder.respondBrokeredWithControl(
		ctx, decoded, tools,
		func(
			callContext context.Context,
			name string,
			arguments json.RawMessage,
		) (json.RawMessage, error) {
			return harnessadapter.CallHarnessControlTool(
				callContext, lease, name, arguments,
			)
		},
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

func productProviderConversationControlTools(
	registry *controltool.Registry,
	lease harnessadapter.HarnessControlMCPLease,
) ([]provider.ConversationControlTool, error) {
	if registry == nil || lease.URL == "" || lease.Token == "" {
		return nil, api.ErrLocalProductChatUnavailable
	}
	definitions := registry.Definitions()
	if len(definitions) == 0 || len(definitions) != len(lease.ToolNames) {
		return nil, api.ErrLocalProductChatUnavailable
	}
	tools := make([]provider.ConversationControlTool, len(definitions))
	for index, definition := range definitions {
		if definition.MCPName != lease.ToolNames[index] {
			return nil, api.ErrLocalProductChatUnavailable
		}
		tools[index] = provider.ConversationControlTool{
			Name: definition.MCPName, Description: definition.Description,
			InputSchema:      append(json.RawMessage(nil), definition.InputSchema...),
			StopAfterSuccess: definition.Effect == controltool.EffectProposal,
		}
	}
	return tools, nil
}

func (session *productLoomNativeSegmentSession) Close(ctx context.Context) error {
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
