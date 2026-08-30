package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/controltool"
	"loom-pi-rebuild/internal/harnessgateway"
	"loom-pi-rebuild/internal/runtime/harnessadapter"
)

const productCodexSegmentBackendID = harnessgateway.BackendID("backend.codex.app-server")

type productCodexSegmentRuntime interface {
	Respond(context.Context, []byte) (harnessadapter.HarnessProcessResult, error)
	Healthy() bool
	Close(context.Context) error
}

type productCodexSegmentSessionOpener func(
	context.Context,
	harnessadapter.CodexSegmentSessionConfig,
) (productCodexSegmentRuntime, error)

type productCodexSegmentBackendConfig struct {
	ExecutablePath  string
	HomePath        string
	PrivateRoot     string
	Timeout         time.Duration
	MaxOutputBytes  int
	Sessions        harnessadapter.HarnessSessionRunner
	Open            productCodexSegmentSessionOpener
	ControlRegistry *controltool.Registry
	ControlGateway  controltool.Gateway
}

type productCodexSegmentBackend struct {
	config productCodexSegmentBackendConfig
}

func newProductCodexSegmentBackend(
	config productCodexSegmentBackendConfig,
) (*productCodexSegmentBackend, error) {
	if !productCodexSegmentCleanAbsolutePath(config.ExecutablePath) ||
		!productCodexSegmentCleanAbsolutePath(config.HomePath) ||
		!productCodexSegmentCleanAbsolutePath(config.PrivateRoot) || config.Sessions == nil {
		return nil, api.ErrLocalProductChatUnavailable
	}
	if config.Timeout == 0 {
		config.Timeout = 8 * time.Hour
	}
	if config.MaxOutputBytes == 0 {
		config.MaxOutputBytes = 64 << 10
	}
	if config.Timeout <= 0 || config.Timeout > 24*time.Hour ||
		config.MaxOutputBytes < 256 || config.MaxOutputBytes > 1<<20 {
		return nil, api.ErrLocalProductChatUnavailable
	}
	if config.Open == nil {
		config.Open = func(
			ctx context.Context,
			segmentConfig harnessadapter.CodexSegmentSessionConfig,
		) (productCodexSegmentRuntime, error) {
			return harnessadapter.OpenCodexSegmentSession(ctx, segmentConfig)
		}
	}
	return &productCodexSegmentBackend{config: config}, nil
}

func (*productCodexSegmentBackend) ID() harnessgateway.BackendID {
	return productCodexSegmentBackendID
}

func (*productCodexSegmentBackend) Version() int {
	return productHarnessGatewayBackendVersion
}

func (backend *productCodexSegmentBackend) OpenSession(
	ctx context.Context,
	configured harnessgateway.ConfiguredHarness,
	binding harnessgateway.SegmentSessionBinding,
	workspace harnessgateway.Workspace,
) (harnessgateway.BackendSession, error) {
	if backend == nil || ctx == nil || ctx.Err() != nil ||
		configured.HarnessID != harnessgateway.HarnessCodex ||
		configured.BackendID != backend.ID() || configured.BackendVersion != backend.Version() ||
		binding.ConfiguredHarnessID != harnessgateway.HarnessCodex ||
		binding.BackendID != backend.ID() || binding.BackendVersion != backend.Version() ||
		workspace.ID != binding.WorkspaceID || workspace.Digest != binding.WorkspaceDigest ||
		!productCodexSegmentCleanAbsolutePath(workspace.Path) {
		return nil, api.ErrLocalProductChatUnavailable
	}
	sessionID := binding.SessionID()
	if sessionID == "" {
		return nil, api.ErrLocalProductChatUnavailable
	}
	systemPrompt, err := productConversationControlSystemPrompt(
		binding.ProviderID, binding.ModelID, "codex", backend.config.ControlRegistry,
	)
	if err != nil {
		return nil, errors.Join(api.ErrLocalProductChatUnavailable, err)
	}
	privateRoot := filepath.Join(backend.config.PrivateRoot, sessionID)
	if err := prepareProductHarnessGatewayWorkspace(privateRoot); err != nil {
		return nil, err
	}
	runtime, err := backend.config.Open(
		ctx,
		harnessadapter.CodexSegmentSessionConfig{
			ExecutablePath: backend.config.ExecutablePath,
			HomePath:       backend.config.HomePath, WorkspacePath: workspace.Path,
			PrivateRoot: privateRoot, ModelID: binding.ModelID,
			ReasoningEffort: binding.ReasoningEffort,
			SystemPrompt:    systemPrompt,
			Timeout:         backend.config.Timeout, MaxOutputBytes: backend.config.MaxOutputBytes,
			Sessions:        backend.config.Sessions,
			ControlRegistry: backend.config.ControlRegistry,
			ControlGateway:  backend.config.ControlGateway,
		},
	)
	if err != nil || runtime == nil {
		_ = os.Remove(privateRoot)
		return nil, errors.Join(api.ErrLocalProductChatUnavailable, err)
	}
	return &productCodexSegmentBackendSession{
		binding: binding, runtime: runtime, privateRoot: privateRoot,
		controlEnabled: backend.config.ControlRegistry != nil,
	}, nil
}

type productCodexSegmentControlRuntime interface {
	BeginControlTurn(controltool.TurnContext) error
	EndControlTurn() (controltool.ProposalBatch, error)
}

type productCodexSegmentBackendSession struct {
	binding        harnessgateway.SegmentSessionBinding
	runtime        productCodexSegmentRuntime
	privateRoot    string
	controlEnabled bool
	mu             sync.Mutex
	turns          int
	closed         bool
}

func (session *productCodexSegmentBackendSession) Respond(
	ctx context.Context,
	request harnessgateway.ResponseRequest,
) (harnessgateway.Response, error) {
	if session == nil || ctx == nil || ctx.Err() != nil {
		return harnessgateway.Response{}, api.ErrLocalProductChatUnavailable
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed || session.runtime == nil {
		return harnessgateway.Response{}, api.ErrLocalProductChatUnavailable
	}
	var decoded api.LocalProductConversationRequest
	if json.Unmarshal(request.Input, &decoded) != nil ||
		!productHarnessGatewayRequestMatches(decoded, request.Authority, session.binding) {
		return harnessgateway.Response{}, api.ErrLocalProductChatUnavailable
	}
	prompt, err := productCodexSegmentPrompt(decoded, session.turns == 0)
	if err != nil {
		return harnessgateway.Response{}, err
	}
	var result harnessadapter.HarnessProcessResult
	var proposals controltool.ProposalBatch
	if session.controlEnabled {
		controlRuntime, ok := session.runtime.(productCodexSegmentControlRuntime)
		if !ok || controlRuntime == nil {
			return harnessgateway.Response{}, api.ErrLocalProductChatUnavailable
		}
		if err := controlRuntime.BeginControlTurn(
			productHarnessControlTurnContext(decoded, session.binding),
		); err != nil {
			return harnessgateway.Response{}, api.ErrLocalProductChatUnavailable
		}
		var respondErr error
		result, respondErr = session.runtime.Respond(ctx, []byte(prompt))
		var endErr error
		proposals, endErr = controlRuntime.EndControlTurn()
		if respondErr != nil {
			err := respondErr
			failure := productCodexSegmentFailure(err)
			if !session.runtime.Healthy() {
				failure = errors.Join(harnessgateway.ErrSessionUnhealthy, failure)
			}
			return harnessgateway.Response{}, failure
		}
		if endErr != nil {
			return harnessgateway.Response{}, api.ErrLocalProductChatUnavailable
		}
	} else {
		var err error
		result, err = session.runtime.Respond(ctx, []byte(prompt))
		if err != nil {
			failure := productCodexSegmentFailure(err)
			if !session.runtime.Healthy() {
				failure = errors.Join(harnessgateway.ErrSessionUnhealthy, failure)
			}
			return harnessgateway.Response{}, failure
		}
	}
	response := api.LocalProductConversationResponse{
		Content: result.Content, Tentative: true,
		ControlProposals:      proposals.SessionAlignments,
		ActionProposals:       proposals.ConversationActions,
		CompletedControlTools: proposals.CompletedCalls,
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
		return harnessgateway.Response{}, api.ErrLocalProductChatUnavailable
	}
	session.turns++
	return harnessgateway.Response{Content: payload}, nil
}

func (session *productCodexSegmentBackendSession) Close(ctx context.Context) error {
	if session == nil || ctx == nil {
		return api.ErrLocalProductChatUnavailable
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed {
		return nil
	}
	session.closed = true
	var result error
	if session.runtime != nil {
		result = session.runtime.Close(ctx)
		session.runtime = nil
	}
	if err := os.Remove(session.privateRoot); err != nil && !errors.Is(err, os.ErrNotExist) {
		result = errors.Join(result, api.ErrLocalProductChatUnavailable)
	}
	return result
}

func productCodexSegmentPrompt(
	request api.LocalProductConversationRequest,
	first bool,
) (string, error) {
	if first {
		return productCodexConversationPrompt(request.ContextPrompt, request.Messages)
	}
	for index := len(request.Messages) - 1; index >= 0; index-- {
		message := request.Messages[index]
		if message.Role != "user" {
			continue
		}
		content := strings.TrimSpace(message.Content)
		if content == "" || len(content) > 60*1024 || !utf8.ValidString(content) ||
			strings.IndexByte(content, 0) >= 0 {
			return "", api.ErrLocalProductChatUnavailable
		}
		return content, nil
	}
	return "", api.ErrLocalProductChatUnavailable
}

func productCodexSegmentFailure(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, harnessadapter.ErrHarnessProviderAuth) {
		return api.NewLocalProductConversationDispatchErrorWithDetails(
			api.LocalProductConversationDispatchFailureInfo{
				Code: "provider_auth", Stage: "provider_auth", Retryable: false,
				ProviderCode: "codex_auth_required",
				UserMessage:  "Codex needs you to sign in again. Open Runtime & Providers, connect Codex, then retry this turn.",
			},
			errors.Join(api.ErrLocalProductChatUnavailable, err),
		)
	}
	if errors.Is(err, harnessadapter.ErrHarnessProviderRateLimit) {
		return api.NewLocalProductConversationDispatchErrorWithDetails(
			api.LocalProductConversationDispatchFailureInfo{
				Code: "provider_rate_limit", Stage: "provider_rate_limit", Retryable: true,
				ProviderCode: "codex_usage_limit",
				UserMessage:  "Codex has reached its current usage window. Retry after the account limit resets or choose another available Runtime.",
			},
			errors.Join(api.ErrLocalProductChatUnavailable, err),
		)
	}
	if errors.Is(err, harnessadapter.ErrHarnessProviderUnavailable) {
		return api.NewLocalProductConversationDispatchErrorWithDetails(
			api.LocalProductConversationDispatchFailureInfo{
				Code: "provider_unavailable", Stage: "provider_connect", Retryable: true,
				ProviderCode: "codex_server_unavailable",
				UserMessage:  "Codex is temporarily unavailable. Retry this turn without changing the frozen Segment binding.",
			},
			errors.Join(api.ErrLocalProductChatUnavailable, err),
		)
	}
	providerCode := ""
	userMessage := "The Codex conversation runtime stopped. Retry to reopen this Segment session."
	if stage := harnessadapter.CodexControlArbitrationFailureStage(err); stage != "" {
		providerCode = "codex_control_" + stage
		userMessage = "Codex could not complete the Loom tool turn. Retry to reopen this Segment session."
	}
	return api.NewLocalProductConversationDispatchErrorWithDetails(
		api.LocalProductConversationDispatchFailureInfo{
			Code: "conversation_unavailable", Stage: "conversation_dispatch", Retryable: true,
			ProviderCode: providerCode, UserMessage: userMessage,
		},
		errors.Join(api.ErrLocalProductChatUnavailable, err),
	)
}

func productCodexSegmentCleanAbsolutePath(path string) bool {
	return filepath.IsAbs(path) && filepath.Clean(path) == path && len(path) <= 4_096
}
