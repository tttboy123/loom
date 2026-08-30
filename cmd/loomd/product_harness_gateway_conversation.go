package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/controltool"
	"loom-pi-rebuild/internal/harnessgateway"
)

const (
	productHarnessGatewayConfiguredVersion     = 1
	productHarnessGatewayBackendVersion        = 1
	productHarnessGatewayMaximumOutput         = 64 << 10
	productHarnessGatewayMaximumVisibleContent = 4_096
	productHarnessGatewayTruncationMarker      = "[Response truncated by Loom. Ask for a narrower result to see more.]"
	productHarnessGatewaySessionOpenLimit      = 30 * time.Second
	productHarnessGatewaySessionCloseLimit     = 4 * time.Second
)

type productHarnessGatewayConversationConfig struct {
	Executor          api.LocalProductConversationResponder
	WorkspacePath     string
	Events            harnessgateway.EventSink
	Now               func() time.Time
	CodexSegment      *productCodexSegmentBackendConfig
	ClaudeCodeSegment *productClaudeCodeSegmentBackendConfig
	OpenCodeSegment   *productOpenCodeSegmentBackendConfig
	PiSegment         *productPiSegmentBackendConfig
	LoomNativeSegment *productLoomNativeSegmentBackendConfig
}

type productHarnessGatewayConversationResponder struct {
	gateway         *harnessgateway.Gateway
	workspace       harnessgateway.Workspace
	backendIDs      map[harnessgateway.HarnessID]harnessgateway.BackendID
	backendVersions map[harnessgateway.HarnessID]int
	activeMu        sync.Mutex
	active          map[string]productHarnessGatewayActiveResponse
}

type productHarnessGatewayActiveResponse struct {
	sessionID  string
	responseID string
	incidentID string
	cancel     context.CancelFunc
}

func productHarnessControlTurnContext(
	request api.LocalProductConversationRequest,
	binding harnessgateway.SegmentSessionBinding,
) controltool.TurnContext {
	route := controltool.FrozenRouteReference{}
	if request.ExecutionBinding != nil {
		route = controltool.FrozenRouteReference{
			HarnessAdapter:         request.ExecutionBinding.HarnessAdapter,
			ProviderID:             request.ExecutionBinding.ProviderID,
			ProviderAccountID:      request.ExecutionBinding.ProviderAccountID,
			CredentialRevision:     request.ExecutionBinding.CredentialRevision,
			ModelID:                request.ExecutionBinding.ModelID,
			ReasoningEffort:        request.ReasoningEffort,
			ExecutionBindingDigest: request.SegmentBindingDigest,
			ContextCapsuleDigest:   request.ContextCapsuleDigest,
		}
	}
	return controltool.TurnContext{
		ConversationID: request.ThreadID, SegmentID: request.SegmentID,
		AttemptID: request.AttemptID, IncidentID: request.IncidentID,
		Catalog:       append([]controltool.SessionReference(nil), request.SessionCatalog...),
		CatalogDigest: request.CatalogDigest,
		Route:         route,
		Workspace: controltool.FrozenWorkspaceReference{
			WorkspaceID: binding.WorkspaceID, WorkspaceDigest: binding.WorkspaceDigest,
		},
	}
}

type productHarnessGatewayOperationalEventSink struct {
	diagnostics productOperationalDiagnosticSink
}

func (sink *productHarnessGatewayOperationalEventSink) Record(
	event harnessgateway.Event,
) {
	if sink == nil || sink.diagnostics == nil || !event.Valid() {
		return
	}
	incidentID := event.IncidentID
	if incidentID == "" {
		incidentID = "loom-session-" + productHarnessGatewayDigest(event.SessionID)[:48]
	}
	result, errorCode, retryable, ok := productHarnessGatewayDiagnosticOutcome(event.Type)
	if !ok {
		return
	}
	stage := "conversation_dispatch"
	httpStatus := 0
	providerCode := ""
	retryAfterSeconds := int64(0)
	if event.Type == harnessgateway.EventResponseFailed {
		stage = event.Failure.Stage
		errorCode = event.Failure.Code
		httpStatus = event.Failure.HTTPStatus
		providerCode = event.Failure.ProviderCode
		retryAfterSeconds = event.Failure.RetryAfterSeconds
		retryable = event.Failure.Retryable
	}
	_ = sink.diagnostics.append(productOperationalDiagnosticRecord{
		SchemaVersion: 1,
		OccurredAt:    event.OccurredAt.UTC().Format(time.RFC3339Nano),
		IncidentID:    incidentID, Operation: "chat_message",
		CredentialRuntime:               sink.diagnostics.credentialRuntimeValue(),
		ThreadID:                        event.ConversationID,
		GatewayEventSchemaVersion:       event.SchemaVersion,
		GatewayInstanceID:               event.GatewayInstanceID,
		GatewayConfiguredHarnessVersion: event.ConfiguredHarnessVersion,
		GatewayBackendVersion:           event.BackendVersion,
		GatewayEventSequence:            event.Sequence,
		GatewayEventType:                string(event.Type),
		SessionID:                       event.SessionID,
		HarnessID:                       string(event.HarnessID),
		BackendID:                       string(event.BackendID),
		SegmentID:                       event.SegmentID,
		WorkspaceID:                     event.WorkspaceID,
		WorkspaceDigest:                 event.WorkspaceDigest,
		ExecutionBindingDigest:          event.ExecutionBindingDigest,
		ProviderID:                      event.ProviderID,
		ProviderAccountID:               event.ProviderAccountID,
		CredentialRevision:              event.CredentialRevision,
		ModelID:                         event.ModelID,
		ReasoningEffort:                 event.ReasoningEffort,
		SegmentContextCapsuleDigest:     event.SegmentContextCapsuleDigest,
		CapsuleDigest:                   event.ContextCapsuleDigest,
		GovernancePolicyDigest:          event.GovernancePolicyDigest,
		RouteTransitionReviewDigest:     event.RouteTransitionReviewDigest,
		ContextAlignmentDigest:          event.ContextAlignmentDigest,
		ResponseID:                      event.ResponseID,
		Stage:                           stage,
		Result:                          result,
		ErrorCode:                       errorCode,
		HTTPStatus:                      httpStatus,
		ProviderErrorCode:               providerCode,
		RetryAfterSeconds:               retryAfterSeconds,
		Retryable:                       retryable,
	})
}

func productHarnessGatewayClassifyResponseFailure(
	err error,
) (harnessgateway.ResponseFailure, bool) {
	failure, ok := api.LocalProductConversationDispatchFailureDetails(err)
	if !ok {
		return harnessgateway.ResponseFailure{}, false
	}
	return harnessgateway.ResponseFailure{
		Code: failure.Code, Stage: failure.Stage,
		HTTPStatus: failure.HTTPStatus, ProviderCode: failure.ProviderCode,
		RetryAfterSeconds: failure.RetryAfterSeconds, Retryable: failure.Retryable,
	}, true
}

func newProductHarnessGatewayConversationResponder(
	config productHarnessGatewayConversationConfig,
) (*productHarnessGatewayConversationResponder, error) {
	if config.Executor == nil || config.Events == nil || config.Now == nil ||
		!filepath.IsAbs(config.WorkspacePath) ||
		filepath.Clean(config.WorkspacePath) != config.WorkspacePath {
		return nil, api.ErrLocalProductChatUnavailable
	}
	workspaceDigest := productHarnessGatewayDigest(config.WorkspacePath)
	workspace := harnessgateway.Workspace{
		ID: "workspace-" + workspaceDigest[:32], Digest: workspaceDigest,
		Path: config.WorkspacePath,
	}
	registrations := make([]harnessgateway.Registration, 0, 5)
	backendIDs := make(map[harnessgateway.HarnessID]harnessgateway.BackendID, 5)
	backendVersions := make(map[harnessgateway.HarnessID]int, 5)
	for _, harnessID := range harnessgateway.BuiltInHarnessIDs() {
		governed, err := newProductGovernedSegmentBackend(harnessID, config.Executor)
		if err != nil {
			return nil, errors.Join(api.ErrLocalProductChatUnavailable, err)
		}
		var backend harnessgateway.Backend = governed
		if harnessID == harnessgateway.HarnessCodex && config.CodexSegment != nil {
			backend, err = newProductCodexSegmentBackend(*config.CodexSegment)
			if err != nil {
				return nil, errors.Join(api.ErrLocalProductChatUnavailable, err)
			}
		}
		if harnessID == harnessgateway.HarnessClaudeCode && config.ClaudeCodeSegment != nil {
			backend, err = newProductClaudeCodeSegmentBackend(*config.ClaudeCodeSegment)
			if err != nil {
				return nil, errors.Join(api.ErrLocalProductChatUnavailable, err)
			}
		}
		if harnessID == harnessgateway.HarnessOpenCode && config.OpenCodeSegment != nil {
			backend, err = newProductOpenCodeSegmentBackend(*config.OpenCodeSegment)
			if err != nil {
				return nil, errors.Join(api.ErrLocalProductChatUnavailable, err)
			}
		}
		if harnessID == harnessgateway.HarnessPi && config.PiSegment != nil {
			backend, err = newProductPiSegmentBackend(*config.PiSegment)
			if err != nil {
				return nil, errors.Join(api.ErrLocalProductChatUnavailable, err)
			}
		}
		if harnessID == harnessgateway.HarnessLoomNative && config.LoomNativeSegment != nil {
			backend, err = newProductLoomNativeSegmentBackend(*config.LoomNativeSegment)
			if err != nil {
				return nil, errors.Join(api.ErrLocalProductChatUnavailable, err)
			}
		}
		backendIDs[harnessID] = backend.ID()
		backendVersions[harnessID] = backend.Version()
		registrations = append(registrations, harnessgateway.Registration{
			Configured: harnessgateway.ConfiguredHarness{
				SchemaVersion: harnessgateway.ConfiguredHarnessSchemaVersion,
				HarnessID:     harnessID, Version: productHarnessGatewayConfiguredVersion,
				BackendID: backend.ID(), BackendVersion: backend.Version(),
				ConfigurationDigest: productHarnessGatewayDigest(
					"configured:" + string(harnessID) + ":" + string(backend.ID()) + ":v1",
				),
				Capabilities: []harnessgateway.Capability{
					harnessgateway.CapabilitySegmentSession,
					harnessgateway.CapabilityResponseCancel,
				},
				MaxConcurrentSessions: 64,
				IdleTimeout:           15 * time.Minute, MaxSessionAge: 8 * time.Hour,
			},
			Backend: backend,
		})
	}
	registry, err := harnessgateway.NewBuiltInBackendRegistry(registrations)
	if err != nil {
		return nil, errors.Join(api.ErrLocalProductChatUnavailable, err)
	}
	gateway, err := harnessgateway.New(harnessgateway.Config{
		Registry: registry, Events: config.Events, Now: config.Now,
		SessionOpenTimeout:  productHarnessGatewaySessionOpenLimit,
		SessionCloseTimeout: productHarnessGatewaySessionCloseLimit,
		ClassifyFailure:     productHarnessGatewayClassifyResponseFailure,
	})
	if err != nil {
		return nil, errors.Join(api.ErrLocalProductChatUnavailable, err)
	}
	return &productHarnessGatewayConversationResponder{
		gateway: gateway, workspace: workspace, backendIDs: backendIDs,
		backendVersions: backendVersions,
		active:          make(map[string]productHarnessGatewayActiveResponse),
	}, nil
}

func (responder *productHarnessGatewayConversationResponder) Respond(
	ctx context.Context,
	request api.LocalProductConversationRequest,
) (api.LocalProductConversationResponse, error) {
	if responder == nil || responder.gateway == nil || ctx == nil || ctx.Err() != nil ||
		request.ExecutionBinding == nil || request.AttemptID == "" ||
		request.ThreadID == "" || request.SegmentID == "" ||
		request.SegmentBindingDigest == "" || request.ContextCapsuleDigest == "" ||
		request.SegmentContextCapsuleDigest == "" ||
		request.ModelID != request.ExecutionBinding.ModelID {
		return api.LocalProductConversationResponse{}, api.ErrLocalProductChatUnavailable
	}
	if request.IncidentID == "" {
		request.IncidentID = "loom-chat-" + productHarnessGatewayDigest(
			request.ThreadID + ":" + request.AttemptID,
		)[:40]
	}
	harnessID, ok := productHarnessGatewayHarnessID(
		request.ExecutionBinding.HarnessAdapter,
	)
	if !ok {
		return api.LocalProductConversationResponse{}, api.ErrLocalProductChatUnavailable
	}
	backendID := responder.backendIDs[harnessID]
	backendVersion := responder.backendVersions[harnessID]
	if backendID == "" || backendVersion < 1 {
		return api.LocalProductConversationResponse{}, api.ErrLocalProductChatUnavailable
	}
	binding := harnessgateway.SegmentSessionBinding{
		SchemaVersion:            harnessgateway.SegmentSessionBindingSchemaVersion,
		ConfiguredHarnessID:      harnessID,
		ConfiguredHarnessVersion: productHarnessGatewayConfiguredVersion,
		BackendID:                backendID, BackendVersion: backendVersion,
		ConversationID: request.ThreadID, SegmentID: request.SegmentID,
		WorkspaceID: responder.workspace.ID, WorkspaceDigest: responder.workspace.Digest,
		ExecutionBindingDigest:      request.SegmentBindingDigest,
		ProviderID:                  request.ExecutionBinding.ProviderID,
		ProviderAccountID:           request.ExecutionBinding.ProviderAccountID,
		CredentialRevision:          request.ExecutionBinding.CredentialRevision,
		ModelID:                     request.ExecutionBinding.ModelID,
		ReasoningEffort:             request.ReasoningEffort,
		SegmentContextCapsuleDigest: request.SegmentContextCapsuleDigest,
		GovernancePolicyDigest:      request.ExecutionBinding.ProviderAccountPolicyDigest,
		RouteTransitionReviewDigest: request.RouteTransitionReviewDigest,
		ContextAlignmentDigest:      request.ContextAlignmentDigest,
	}
	responseContext, cancelResponse := context.WithCancel(ctx)
	active := productHarnessGatewayActiveResponse{
		sessionID: binding.SessionID(), responseID: request.AttemptID,
		incidentID: request.IncidentID, cancel: cancelResponse,
	}
	activeKey := productHarnessGatewayActiveResponseKey(request.ThreadID, request.IncidentID)
	if active.sessionID == "" || !responder.registerActive(activeKey, active) {
		cancelResponse()
		return api.LocalProductConversationResponse{}, harnessgateway.ErrAuthorityConflict
	}
	defer func() {
		cancelResponse()
		responder.unregisterActive(activeKey, active)
	}()
	payload, err := json.Marshal(request)
	if err != nil {
		return api.LocalProductConversationResponse{}, api.ErrLocalProductChatUnavailable
	}
	defer clearProductHarnessGatewayBytes(payload)
	response, err := responder.gateway.Respond(
		responseContext, binding, responder.workspace,
		harnessgateway.ResponseRequest{
			Authority: harnessgateway.ResponseAuthority{
				SchemaVersion: harnessgateway.ResponseAuthoritySchemaVersion,
				ResponseID:    request.AttemptID, IncidentID: request.IncidentID,
				ExecutionBindingDigest:      binding.ExecutionBindingDigest,
				ContextCapsuleDigest:        request.ContextCapsuleDigest,
				SegmentContextCapsuleDigest: binding.SegmentContextCapsuleDigest,
				GovernancePolicyDigest:      binding.GovernancePolicyDigest,
				RouteTransitionReviewDigest: binding.RouteTransitionReviewDigest,
				ContextAlignmentDigest:      binding.ContextAlignmentDigest,
				ProviderID:                  binding.ProviderID, ProviderAccountID: binding.ProviderAccountID,
				CredentialRevision: binding.CredentialRevision, ModelID: binding.ModelID,
				ReasoningEffort: binding.ReasoningEffort,
			},
			Input: payload,
		},
	)
	if err != nil {
		if responseContext.Err() != nil {
			return api.LocalProductConversationResponse{}, responseContext.Err()
		}
		return api.LocalProductConversationResponse{}, err
	}
	defer clearProductHarnessGatewayBytes(response.Content)
	var decoded api.LocalProductConversationResponse
	if json.Unmarshal(response.Content, &decoded) != nil ||
		!validProductHarnessGatewayResponse(decoded) {
		return api.LocalProductConversationResponse{}, api.ErrLocalProductChatUnavailable
	}
	return decoded, nil
}

func (responder *productHarnessGatewayConversationResponder) CancelChatResponse(
	ctx context.Context,
	request api.LocalProductChatResponseCancelRequest,
) error {
	if responder == nil || responder.gateway == nil || ctx == nil || ctx.Err() != nil ||
		request.ThreadID == "" || request.IncidentID == "" {
		return api.ErrInvalidLocalProductChatRequest
	}
	key := productHarnessGatewayActiveResponseKey(request.ThreadID, request.IncidentID)
	responder.activeMu.Lock()
	active, found := responder.active[key]
	responder.activeMu.Unlock()
	if !found || active.incidentID != request.IncidentID || active.cancel == nil {
		return harnessgateway.ErrResponseNotFound
	}
	active.cancel()
	err := responder.gateway.CancelResponse(ctx, harnessgateway.CancelRequest{
		SessionID: active.sessionID, ResponseID: active.responseID,
		IncidentID: active.incidentID,
	})
	return err
}

func (responder *productHarnessGatewayConversationResponder) registerActive(
	key string,
	active productHarnessGatewayActiveResponse,
) bool {
	responder.activeMu.Lock()
	defer responder.activeMu.Unlock()
	if responder.active == nil {
		return false
	}
	if _, duplicate := responder.active[key]; duplicate {
		return false
	}
	responder.active[key] = active
	return true
}

func (responder *productHarnessGatewayConversationResponder) unregisterActive(
	key string,
	active productHarnessGatewayActiveResponse,
) {
	responder.activeMu.Lock()
	if current, found := responder.active[key]; found &&
		current.sessionID == active.sessionID && current.responseID == active.responseID {
		delete(responder.active, key)
	}
	responder.activeMu.Unlock()
}

func productHarnessGatewayActiveResponseKey(threadID, incidentID string) string {
	return threadID + "\x00" + incidentID
}

func (responder *productHarnessGatewayConversationResponder) Close(
	ctx context.Context,
) error {
	if responder == nil || responder.gateway == nil {
		return nil
	}
	return responder.gateway.Close(ctx)
}

func productHarnessGatewayRequestMatches(
	request api.LocalProductConversationRequest,
	authority harnessgateway.ResponseAuthority,
	binding harnessgateway.SegmentSessionBinding,
) bool {
	if request.ExecutionBinding == nil || request.ThreadID != binding.ConversationID ||
		request.SegmentID != binding.SegmentID || request.AttemptID != authority.ResponseID ||
		request.IncidentID != authority.IncidentID ||
		request.SegmentBindingDigest != binding.ExecutionBindingDigest ||
		request.SegmentContextCapsuleDigest != binding.SegmentContextCapsuleDigest ||
		request.ContextCapsuleDigest != authority.ContextCapsuleDigest ||
		request.ExecutionBinding.HarnessAdapter != string(binding.ConfiguredHarnessID) ||
		request.ExecutionBinding.ProviderID != binding.ProviderID ||
		request.ExecutionBinding.ModelID != binding.ModelID ||
		request.ReasoningEffort != binding.ReasoningEffort {
		return false
	}
	return request.ExecutionBinding.ProviderAccountID == binding.ProviderAccountID &&
		request.ExecutionBinding.CredentialRevision == binding.CredentialRevision &&
		request.ExecutionBinding.ProviderAccountPolicyDigest == binding.GovernancePolicyDigest &&
		request.RouteTransitionReviewDigest == binding.RouteTransitionReviewDigest &&
		request.ContextAlignmentDigest == binding.ContextAlignmentDigest
}

func productHarnessGatewayHarnessID(
	adapter string,
) (harnessgateway.HarnessID, bool) {
	for _, harnessID := range harnessgateway.BuiltInHarnessIDs() {
		if adapter == string(harnessID) {
			return harnessID, true
		}
	}
	return "", false
}

func productHarnessGatewayDigest(value string) string {
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func prepareProductHarnessGatewayWorkspace(path string) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return api.ErrLocalProductChatUnavailable
	}
	if err := os.MkdirAll(path, 0o700); err != nil {
		return api.ErrLocalProductChatUnavailable
	}
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return api.ErrLocalProductChatUnavailable
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(stat.Uid) != os.Getuid() {
		return api.ErrLocalProductChatUnavailable
	}
	if info.Mode().Perm() != 0o700 {
		if err := os.Chmod(path, 0o700); err != nil {
			return api.ErrLocalProductChatUnavailable
		}
	}
	return nil
}

func validProductHarnessGatewayResponse(
	response api.LocalProductConversationResponse,
) bool {
	content := strings.TrimSpace(response.Content)
	if content == "" || len(content) > productHarnessGatewayMaximumVisibleContent ||
		!utf8.ValidString(content) ||
		strings.IndexByte(content, 0) >= 0 || len(response.ControlProposals) > 8 ||
		len(response.ActionProposals) > 8 {
		return false
	}
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil || !registry.ValidCompletedCalls(response.CompletedControlTools, 8) {
		return false
	}
	completedProposals := make(map[controltool.ToolID]int)
	for _, call := range response.CompletedControlTools {
		if call.Effect == controltool.EffectProposal {
			completedProposals[call.ToolID]++
		}
	}
	capturedProposals := make(map[controltool.ToolID]int)
	seen := make(
		map[string]struct{}, len(response.ControlProposals)+len(response.ActionProposals),
	)
	for _, proposal := range response.ControlProposals {
		if !proposal.Valid() || proposal.MessageID != "" {
			return false
		}
		if _, duplicate := seen[proposal.ProposalDigest]; duplicate {
			return false
		}
		seen[proposal.ProposalDigest] = struct{}{}
		capturedProposals[proposal.ToolID]++
	}
	for _, proposal := range response.ActionProposals {
		if !proposal.Valid() || proposal.MessageID != "" {
			return false
		}
		if _, duplicate := seen[proposal.ProposalDigest]; duplicate {
			return false
		}
		seen[proposal.ProposalDigest] = struct{}{}
		capturedProposals[proposal.ToolID]++
	}
	if len(completedProposals) != len(capturedProposals) {
		return false
	}
	for toolID, count := range capturedProposals {
		if completedProposals[toolID] != count {
			return false
		}
	}
	return true
}

func productHarnessGatewayResponseWithProposalFallback(
	response api.LocalProductConversationResponse,
) api.LocalProductConversationResponse {
	content := strings.TrimSpace(response.Content)
	if len(content) > productHarnessGatewayMaximumVisibleContent {
		prefixBytes := productHarnessGatewayMaximumVisibleContent -
			len(productHarnessGatewayTruncationMarker) - 2
		for prefixBytes > 0 && !utf8.ValidString(content[:prefixBytes]) {
			prefixBytes--
		}
		response.Content = strings.TrimSpace(content[:prefixBytes]) + "\n\n" +
			productHarnessGatewayTruncationMarker
	}
	if strings.TrimSpace(response.Content) == "" &&
		(len(response.ControlProposals) != 0 || len(response.ActionProposals) != 0) {
		response.Content = "I prepared a Loom proposal for your review."
	}
	return response
}

func clearProductHarnessGatewayBytes(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
