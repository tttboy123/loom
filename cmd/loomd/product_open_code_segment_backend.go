package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/controltool"
	"loom-pi-rebuild/internal/credentials"
	credentialvault "loom-pi-rebuild/internal/credentials/vault"
	"loom-pi-rebuild/internal/harnessgateway"
	"loom-pi-rebuild/internal/provider"
	"loom-pi-rebuild/internal/runtime/harnessadapter"
)

const (
	productOpenCodeSegmentBackendID = harnessgateway.BackendID(
		"backend.opencode.run-control",
	)
	productOpenCodeSegmentBackendVersion = 1
	productOpenCodeMaximumControlCalls   = 8
)

var errProductOpenCodeTerminalProposal = errors.New("OpenCode terminal Proposal completed")

type productOpenCodeCredentialResolver func(
	string,
) (credentialvault.CredentialIdentity, bool)

type productOpenCodeSegmentBackendConfig struct {
	ExecutablePath    string
	HomePath          string
	PrivateRoot       string
	Timeout           time.Duration
	MaxOutputBytes    int
	Runner            harnessadapter.HarnessProcessRunner
	CredentialLeases  productCredentialLeaseAccess
	ResolveCredential productOpenCodeCredentialResolver
	ControlRegistry   *controltool.Registry
	ControlGateway    controltool.Gateway
}

type productOpenCodeSegmentBackend struct {
	config productOpenCodeSegmentBackendConfig
}

func newProductOpenCodeSegmentBackend(
	config productOpenCodeSegmentBackendConfig,
) (*productOpenCodeSegmentBackend, error) {
	if !productCodexSegmentCleanAbsolutePath(config.ExecutablePath) ||
		!productCodexSegmentCleanAbsolutePath(config.HomePath) ||
		!productCodexSegmentCleanAbsolutePath(config.PrivateRoot) ||
		nilProductAssetPort(config.Runner) ||
		nilProductAssetPort(config.CredentialLeases) || config.ResolveCredential == nil ||
		config.ControlRegistry == nil || nilProductAssetPort(config.ControlGateway) ||
		config.Timeout <= 0 || config.Timeout > time.Hour ||
		config.MaxOutputBytes < 256 || config.MaxOutputBytes > 1<<20 {
		return nil, api.ErrLocalProductChatUnavailable
	}
	return &productOpenCodeSegmentBackend{config: config}, nil
}

func (*productOpenCodeSegmentBackend) ID() harnessgateway.BackendID {
	return productOpenCodeSegmentBackendID
}

func (*productOpenCodeSegmentBackend) Version() int {
	return productOpenCodeSegmentBackendVersion
}

func (backend *productOpenCodeSegmentBackend) OpenSession(
	ctx context.Context,
	configured harnessgateway.ConfiguredHarness,
	binding harnessgateway.SegmentSessionBinding,
	workspace harnessgateway.Workspace,
) (harnessgateway.BackendSession, error) {
	if backend == nil || ctx == nil || ctx.Err() != nil ||
		configured.HarnessID != harnessgateway.HarnessOpenCode ||
		configured.BackendID != backend.ID() || configured.BackendVersion != backend.Version() ||
		binding.ConfiguredHarnessID != harnessgateway.HarnessOpenCode ||
		binding.BackendID != backend.ID() || binding.BackendVersion != backend.Version() ||
		workspace.ID != binding.WorkspaceID || workspace.Digest != binding.WorkspaceDigest ||
		!productCodexSegmentCleanAbsolutePath(workspace.Path) || binding.SessionID() == "" {
		return nil, api.ErrLocalProductChatUnavailable
	}
	systemPrompt, err := productConversationControlSystemPrompt(
		binding.ProviderID, binding.ModelID, "opencode", backend.config.ControlRegistry,
	)
	if err != nil {
		return nil, errors.Join(api.ErrLocalProductChatUnavailable, err)
	}
	controlLifetime, cancelControl := context.WithCancel(context.Background())
	controlServer, err := harnessadapter.OpenHarnessControlServer(
		controlLifetime, backend.config.ControlRegistry, backend.config.ControlGateway,
	)
	if err != nil {
		cancelControl()
		return nil, errors.Join(api.ErrLocalProductChatUnavailable, err)
	}
	return &productOpenCodeSegmentSession{
		binding: binding, workspacePath: workspace.Path, config: backend.config,
		systemPrompt: systemPrompt, controlServer: controlServer,
		cancelControl: cancelControl,
	}, nil
}

type productOpenCodeSegmentSession struct {
	binding       harnessgateway.SegmentSessionBinding
	workspacePath string
	systemPrompt  string
	config        productOpenCodeSegmentBackendConfig
	controlServer *harnessadapter.HarnessControlServer
	cancelControl context.CancelFunc
	mu            sync.Mutex
	closed        bool
}

func (session *productOpenCodeSegmentSession) Respond(
	ctx context.Context,
	request harnessgateway.ResponseRequest,
) (harnessgateway.Response, error) {
	if session == nil || ctx == nil || ctx.Err() != nil {
		return harnessgateway.Response{}, api.ErrLocalProductChatUnavailable
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closed || session.controlServer == nil {
		return harnessgateway.Response{}, errors.Join(
			harnessgateway.ErrSessionUnhealthy, api.ErrLocalProductChatUnavailable,
		)
	}
	var decoded api.LocalProductConversationRequest
	if json.Unmarshal(request.Input, &decoded) != nil ||
		!productHarnessGatewayRequestMatches(decoded, request.Authority, session.binding) {
		return harnessgateway.Response{}, api.ErrLocalProductChatUnavailable
	}
	if err := session.controlServer.BeginTurn(
		productHarnessControlTurnContext(decoded, session.binding),
	); err != nil {
		return harnessgateway.Response{}, errors.Join(
			harnessgateway.ErrSessionUnhealthy, api.ErrLocalProductChatUnavailable,
		)
	}
	terminalProposal, err := session.controlServer.TerminalProposalCompleted()
	if err != nil {
		_, _ = session.controlServer.EndTurn()
		return harnessgateway.Response{}, errors.Join(
			harnessgateway.ErrSessionUnhealthy, api.ErrLocalProductChatUnavailable,
		)
	}
	turnContext, cancelTurn := context.WithCancelCause(ctx)
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		select {
		case <-terminalProposal:
			if ctx.Err() == nil {
				cancelTurn(errProductOpenCodeTerminalProposal)
			}
		case <-turnContext.Done():
		}
	}()
	response, respondErr := session.respond(
		turnContext, decoded, session.controlServer.Lease(),
	)
	turnCause := context.Cause(turnContext)
	cancelTurn(nil)
	<-watchDone
	proposals, endErr := session.controlServer.EndTurn()
	if endErr != nil {
		return harnessgateway.Response{}, errors.Join(
			harnessgateway.ErrSessionUnhealthy, api.ErrLocalProductChatUnavailable,
		)
	}
	if respondErr != nil && (ctx.Err() != nil ||
		!errors.Is(turnCause, errProductOpenCodeTerminalProposal) ||
		!productOpenCodeTerminalProposalBatchMatches(
			decoded, proposals, session.config.ControlRegistry,
		)) {
		return harnessgateway.Response{}, respondErr
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

func productOpenCodeTerminalProposalBatchMatches(
	request api.LocalProductConversationRequest,
	batch controltool.ProposalBatch,
	registry *controltool.Registry,
) bool {
	if registry == nil || len(batch.CompletedCalls) == 0 ||
		!registry.ValidCompletedCalls(batch.CompletedCalls, productOpenCodeMaximumControlCalls) ||
		len(batch.SessionAlignments)+len(batch.ConversationActions) != 1 {
		return false
	}
	last := batch.CompletedCalls[len(batch.CompletedCalls)-1]
	if last.Effect != controltool.EffectProposal {
		return false
	}
	proposalCalls := 0
	for _, call := range batch.CompletedCalls {
		if call.Effect == controltool.EffectProposal {
			proposalCalls++
		}
	}
	if proposalCalls != 1 {
		return false
	}
	if len(batch.SessionAlignments) == 1 {
		proposal := batch.SessionAlignments[0]
		return proposal.Valid() && proposal.ToolID == last.ToolID &&
			proposal.TargetConversationID == request.ThreadID &&
			proposal.SegmentID == request.SegmentID &&
			proposal.AttemptID == request.AttemptID &&
			proposal.IncidentID == request.IncidentID &&
			proposal.RegistryDigest == registry.Digest()
	}
	proposal := batch.ConversationActions[0]
	return proposal.Valid() && proposal.ToolID == last.ToolID &&
		proposal.TargetConversationID == request.ThreadID &&
		proposal.SegmentID == request.SegmentID &&
		proposal.AttemptID == request.AttemptID &&
		proposal.IncidentID == request.IncidentID &&
		proposal.RegistryDigest == registry.Digest()
}

func (session *productOpenCodeSegmentSession) respond(
	ctx context.Context,
	request api.LocalProductConversationRequest,
	controlMCP harnessadapter.HarnessControlMCPLease,
) (_ api.LocalProductConversationResponse, resultErr error) {
	if request.ExecutionBinding == nil ||
		request.ExecutionBinding.HarnessAdapter != "opencode" ||
		request.ExecutionBinding.ModelID != request.ModelID {
		return api.LocalProductConversationResponse{}, api.ErrLocalProductChatUnavailable
	}
	prompt, err := productCodexConversationPrompt(request.ContextPrompt, request.Messages)
	if err != nil {
		return api.LocalProductConversationResponse{}, err
	}
	tempPath, err := os.MkdirTemp(session.config.PrivateRoot, "response-")
	if err != nil {
		return api.LocalProductConversationResponse{}, api.ErrLocalProductChatUnavailable
	}
	defer func() { resultErr = errors.Join(resultErr, os.RemoveAll(tempPath)) }()
	processRequest := harnessadapter.HarnessProcessRequest{
		ExecutablePath: session.config.ExecutablePath, WorkspacePath: session.workspacePath,
		HomePath: session.config.HomePath, TempPath: tempPath,
		ModelID: request.ModelID, ReasoningEffort: request.ReasoningEffort,
		Prompt: []byte(prompt), SystemPrompt: session.systemPrompt,
		Timeout: session.config.Timeout, MaxOutputBytes: session.config.MaxOutputBytes,
		ControlMCP: controlMCP,
	}
	defer clearProductHarnessGatewayBytes(processRequest.Prompt)
	var result harnessadapter.HarnessProcessResult
	binding := request.ExecutionBinding
	if binding.ProviderAccountID == "" && binding.CredentialRevision == 0 {
		if binding.ProviderID != "opencode" ||
			!validOpenCodeNativeConversationModel(request.ModelID) {
			return api.LocalProductConversationResponse{}, api.ErrLocalProductChatUnavailable
		}
		result, err = session.config.Runner.RunHarness(ctx, processRequest, nil)
	} else {
		identity, found := session.config.ResolveCredential(request.ProfileID)
		if !found || identity.ProviderID != binding.ProviderID ||
			identity.ProviderAccountID != binding.ProviderAccountID ||
			identity.CredentialRevision != binding.CredentialRevision ||
			identity.CredentialReference == "" ||
			!credentials.ValidProviderAccountIdentifier(
				identity.ProviderID, identity.ProviderAccountID,
			) || !openCodeModelMatchesProvider(request.ModelID, identity.ProviderID) {
			return api.LocalProductConversationResponse{}, api.ErrLocalProductChatUnavailable
		}
		processRequest.RequiresCredential = true
		err = session.config.CredentialLeases.UseCredential(
			ctx, identity, func(leaseContext context.Context, secret []byte) error {
				var runErr error
				result, runErr = session.config.Runner.RunHarness(
					leaseContext, processRequest, secret,
				)
				return runErr
			},
		)
	}
	if err != nil {
		return api.LocalProductConversationResponse{}, productOpenCodeSegmentFailure(err)
	}
	content := strings.TrimSpace(result.Content)
	if len(content) > session.config.MaxOutputBytes || !utf8.ValidString(content) ||
		strings.IndexByte(content, 0) >= 0 {
		return api.LocalProductConversationResponse{}, api.ErrLocalProductChatUnavailable
	}
	return api.LocalProductConversationResponse{Content: content, Tentative: true}, nil
}

func productOpenCodeSegmentFailure(err error) error {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	if errors.Is(err, harnessadapter.ErrHarnessProviderTimeout) {
		return api.NewLocalProductConversationDispatchErrorWithDetails(
			api.LocalProductConversationDispatchFailureInfo{
				Code: "timeout", Stage: "provider_connect", Retryable: true,
				ProviderCode: "opencode_turn_timeout",
				UserMessage:  "OpenCode did not finish this turn in time. Retry without changing the frozen Segment binding.",
			},
			errors.Join(api.ErrLocalProductChatUnavailable, err),
		)
	}
	if stage := credentials.CredentialFailureStage(err); stage != "" {
		return api.NewLocalProductConversationDispatchErrorWithDetails(
			api.LocalProductConversationDispatchFailureInfo{
				Code: "credential_unavailable", Stage: stage,
				Retryable:   credentials.CredentialFailureRetryable(stage),
				UserMessage: "The selected OpenCode Provider Account credential is unavailable.",
			},
			errors.Join(api.ErrLocalProductChatUnavailable, err),
		)
	}
	if _, ok := provider.ConversationFailureDetails(err); ok {
		return productOpenCodeConversationFailure(err)
	}
	if errors.Is(err, provider.ErrOpenCodeConversationAuth) ||
		errors.Is(err, provider.ErrOpenCodeConversationInsufficientBalance) ||
		errors.Is(err, provider.ErrOpenCodeConversationModelUnavailable) ||
		errors.Is(err, provider.ErrOpenCodeConversationRateLimit) {
		return productOpenCodeConversationFailure(err)
	}
	return api.NewLocalProductConversationDispatchErrorWithDetails(
		api.LocalProductConversationDispatchFailureInfo{
			Code: "conversation_unavailable", Stage: "conversation_dispatch", Retryable: true,
			UserMessage: "The OpenCode conversation runtime stopped. Retry this turn.",
		},
		errors.Join(api.ErrLocalProductChatUnavailable, err),
	)
}

func (session *productOpenCodeSegmentSession) Close(ctx context.Context) error {
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
