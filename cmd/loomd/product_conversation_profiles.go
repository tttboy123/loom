package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/credentials"
	credentialvault "loom-pi-rebuild/internal/credentials/vault"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/provider"
	"loom-pi-rebuild/internal/work"
)

type productConversationCredentialLookup func(
	string,
) []projection.ProviderCredentialRecord

type productConversationPolicyLookup func(
	string,
	string,
) (work.ProviderAccountPolicy, bool)

type productProviderConversationClient interface {
	Respond(
		context.Context,
		[]provider.ConversationMessage,
		[]byte,
	) (string, error)
	RespondConfigured(
		context.Context,
		[]provider.ConversationMessage,
		[]byte,
		string,
		string,
	) (string, error)
}

func (router *productConversationProfileRouter) ResolveConversationContextTarget(
	ctx context.Context,
	threadID string,
	segmentID string,
	profileID string,
) (contextcapsule.Target, error) {
	if router == nil || ctx == nil || ctx.Err() != nil || threadID == "" ||
		segmentID == "" {
		return contextcapsule.Target{}, api.ErrLocalProductChatUnavailable
	}
	if profileID == "" {
		if _, localPi := router.defaultResponder.(*productPiConversationResponder); localPi {
			return contextcapsule.Target{
				ConversationID: threadID, TeamID: "conversation:" + threadID,
				AgentID: "conversation-agent", RoleID: segmentID,
				ProviderID: "loom-local",
				ModelID:    "qwen2.5-coder-1.5b-instruct-q4-k-m",
				AuthMode:   "native_auth", ContextAdapterID: "context:pi:v1",
				DisclosurePolicyID:      "loom.local-conversation-disclosure",
				DisclosurePolicyVersion: 1, TokenBudget: 8_192,
			}, nil
		}
	}
	if profileID == "" || profileID == provider.CodexConversationProfileID {
		return contextcapsule.Target{
			ConversationID: threadID, TeamID: "conversation:" + threadID,
			AgentID: "conversation-agent", RoleID: segmentID,
			ProviderID: "openai", ModelID: "codex-default", AuthMode: "native_auth",
			ContextAdapterID:        "context:codex:v1",
			DisclosurePolicyID:      "loom.local-conversation-disclosure",
			DisclosurePolicyVersion: 1, TokenBudget: 8_192,
		}, nil
	}
	if profileID == provider.OpenCodeConversationProfileID {
		// OpenCode is a native conversation profile: the bound model (for
		// example deepseek/deepseek-chat) selects which Provider credential is
		// injected at dispatch time, so the capsule records the OpenCode route
		// target and a stable context adapter identity.
		return contextcapsule.Target{
			ConversationID: threadID, TeamID: "conversation:" + threadID,
			AgentID: "conversation-agent", RoleID: segmentID,
			ProviderID: "opencode", ModelID: "opencode-default",
			AuthMode: "native_auth", ContextAdapterID: "context:loom-native:v1",
			DisclosurePolicyID:      "loom.local-conversation-disclosure",
			DisclosurePolicyVersion: 1, TokenBudget: 8_192,
		}, nil
	}
	selected, record, found := router.resolveBrokeredProfile(profileID)
	if !found {
		return contextcapsule.Target{}, api.ErrLocalProductChatUnavailable
	}
	modelID := productConversationModelID(selected.providerID)
	if modelID == "" {
		return contextcapsule.Target{}, api.ErrLocalProductChatUnavailable
	}
	disclosurePolicyID := "loom.local-conversation-disclosure"
	disclosurePolicyVersion := 1
	if policy, ok := router.policy(
		record.ProviderID, record.ProviderAccountID,
	); ok {
		disclosurePolicyID = fmt.Sprintf(
			"provider-account-policy:%s", policy.Digest(),
		)
		disclosurePolicyVersion = policy.Version()
	}
	return contextcapsule.Target{
		ConversationID: threadID, TeamID: "conversation:" + threadID,
		AgentID: "conversation-agent", RoleID: segmentID,
		ProviderID: record.ProviderID, ProviderAccountID: record.ProviderAccountID,
		ModelID: modelID, AuthMode: record.AuthMode,
		ContextAdapterID:        "context:loom-native:v1",
		DisclosurePolicyID:      disclosurePolicyID,
		DisclosurePolicyVersion: disclosurePolicyVersion, TokenBudget: 8_192,
	}, nil
}

func productConversationModelID(providerID string) string {
	switch providerID {
	case "anthropic":
		return provider.AnthropicConversationModelID
	case "deepseek":
		return provider.DeepSeekConversationModelID
	case "kimi":
		return provider.KimiConversationModelID
	case "minimax":
		return provider.MiniMaxConversationModelID
	default:
		return ""
	}
}

// productExternalSessionHandleAccess is the only production boundary for
// Provider-native conversation state. Stateless Provider clients do not use it.
type productExternalSessionHandleAccess interface {
	PutExternalSessionHandle(
		context.Context,
		credentialvault.ExternalSessionHandle,
	) error
	UseExternalSessionHandle(
		context.Context,
		credentialvault.ExternalSessionHandleBinding,
		string,
		func(context.Context, []byte, int64) error,
	) error
}

type productDeepSeekConversationClient = productProviderConversationClient

type productConversationProviderRoute struct {
	providerID string
	profileID  func(string, int64) string
	client     productProviderConversationClient
}

type productConversationProfileRouter struct {
	defaultResponder        api.LocalProductConversationResponder
	codexResponder          api.LocalProductConversationResponder
	credential              productConversationCredentialLookup
	policy                  productConversationPolicyLookup
	leasing                 productCredentialLeaseAccess
	routes                  []productConversationProviderRoute
	requireExecutionBinding bool
}

func newProductConversationProfileRouter(
	defaultResponder api.LocalProductConversationResponder,
	credential productConversationCredentialLookup,
	leasing productCredentialLeaseAccess,
	deepSeek productDeepSeekConversationClient,
	additionalRoutes ...productConversationProviderRoute,
) (*productConversationProfileRouter, error) {
	return newProductConversationProfileRouterWithPolicy(
		defaultResponder, credential,
		func(string, string) (work.ProviderAccountPolicy, bool) {
			return work.ProviderAccountPolicy{}, false
		},
		leasing, deepSeek, false, nil, additionalRoutes...,
	)
}

func newProductConversationProfileRouterWithPolicy(
	defaultResponder api.LocalProductConversationResponder,
	credential productConversationCredentialLookup,
	policy productConversationPolicyLookup,
	leasing productCredentialLeaseAccess,
	deepSeek productDeepSeekConversationClient,
	requireExecutionBinding bool,
	codexResponder api.LocalProductConversationResponder,
	additionalRoutes ...productConversationProviderRoute,
) (*productConversationProfileRouter, error) {
	if credential == nil || policy == nil || leasing == nil || deepSeek == nil {
		return nil, api.ErrLocalProductChatUnavailable
	}
	routes := []productConversationProviderRoute{{
		providerID: "deepseek",
		profileID:  provider.DeepSeekConversationAccountProfileID,
		client:     deepSeek,
	}}
	seenProviders := map[string]struct{}{"deepseek": {}}
	for _, route := range additionalRoutes {
		if route.providerID == "" || route.profileID == nil || route.client == nil {
			return nil, api.ErrLocalProductChatUnavailable
		}
		if _, duplicate := seenProviders[route.providerID]; duplicate {
			return nil, api.ErrLocalProductChatUnavailable
		}
		seenProviders[route.providerID] = struct{}{}
		routes = append(routes, route)
	}
	return &productConversationProfileRouter{
		defaultResponder:        defaultResponder,
		codexResponder:          codexResponder,
		credential:              credential,
		policy:                  policy,
		leasing:                 leasing,
		routes:                  routes,
		requireExecutionBinding: requireExecutionBinding,
	}, nil
}

func (router *productConversationProfileRouter) ResolveConversationExecutionBinding(
	_ context.Context,
	profileID string,
) (api.LocalProductConversationExecutionBinding, error) {
	if router == nil || profileID == "" ||
		profileID == provider.CodexConversationProfileID ||
		profileID == provider.OpenCodeConversationProfileID {
		if router != nil && (profileID == "" ||
			profileID == provider.CodexConversationProfileID ||
			profileID == provider.OpenCodeConversationProfileID) {
			nativeProviderID := "openai"
			if profileID == provider.OpenCodeConversationProfileID {
				nativeProviderID = "opencode"
			}
			return api.LocalProductConversationExecutionBinding{
				SchemaVersion: 3, ProviderID: nativeProviderID,
			}, nil
		}
		return api.LocalProductConversationExecutionBinding{},
			api.ErrLocalProductChatUnavailable
	}
	_, record, found := router.resolveBrokeredProfile(profileID)
	if !found {
		return api.LocalProductConversationExecutionBinding{},
			api.ErrLocalProductChatUnavailable
	}
	binding := api.LocalProductConversationExecutionBinding{
		SchemaVersion: 3, ProviderID: record.ProviderID,
		ProviderAccountID: record.ProviderAccountID,
	}
	if policy, ok := router.policy(
		record.ProviderID, record.ProviderAccountID,
	); ok {
		binding.ProviderAccountPolicyVersion = policy.Version()
		binding.ProviderAccountPolicyRevision = policy.Revision()
		binding.ProviderAccountPolicyDigest = policy.Digest()
		binding.TrustDomain = policy.TrustDomain()
		binding.RetentionMode = policy.RetentionMode()
		binding.DataRegion = policy.DataRegion()
	}
	return binding, nil
}

func (router *productConversationProfileRouter) resolveBrokeredProfile(
	profileID string,
) (productConversationProviderRoute, projection.ProviderCredentialRecord, bool) {
	if router == nil || profileID == "" {
		return productConversationProviderRoute{}, projection.ProviderCredentialRecord{}, false
	}
	var selected productConversationProviderRoute
	var record projection.ProviderCredentialRecord
	found := false
	for _, route := range router.routes {
		for _, candidate := range router.credential(route.providerID) {
			if profileID != route.profileID(
				candidate.ProviderAccountID, candidate.Revision,
			) {
				continue
			}
			if found {
				return productConversationProviderRoute{}, projection.ProviderCredentialRecord{}, false
			}
			selected, record, found = route, candidate, true
		}
	}
	if !found || record.ProviderID != selected.providerID ||
		!credentials.ValidProviderAccountIdentifier(
			record.ProviderID, record.ProviderAccountID,
		) || record.AuthMode != "brokered" || record.Status != "verified" ||
		record.Reason != "" || record.CredentialReference == "" || record.Revision <= 0 {
		return productConversationProviderRoute{}, projection.ProviderCredentialRecord{}, false
	}
	return selected, record, true
}

func (router *productConversationProfileRouter) Respond(
	ctx context.Context,
	request api.LocalProductConversationRequest,
) (api.LocalProductConversationResponse, error) {
	if router == nil || ctx == nil || len(request.Messages) == 0 {
		return api.LocalProductConversationResponse{}, api.ErrLocalProductChatUnavailable
	}
	if request.ProfileID == "" ||
		request.ProfileID == provider.CodexConversationProfileID ||
		request.ProfileID == provider.OpenCodeConversationProfileID {
		if router.requireExecutionBinding {
			wantBinding, bindingErr := router.ResolveConversationExecutionBinding(
				ctx, request.ProfileID,
			)
			if bindingErr != nil || request.ExecutionBinding == nil ||
				*request.ExecutionBinding != wantBinding {
				return api.LocalProductConversationResponse{},
					api.NewLocalProductConversationDispatchError(
						"invalid_request", "conversation_dispatch", false,
						api.ErrLocalProductChatUnavailable,
					)
			}
		}
		// The Codex profile must be served by the Codex responder and the
		// OpenCode profile by the OpenCode responder even when both runtimes
		// are configured; otherwise codex-default would be sent to `opencode
		// run` and fail with an opaque runtime error.
		selectedResponder := router.defaultResponder
		if request.ProfileID == provider.CodexConversationProfileID &&
			router.codexResponder != nil {
			selectedResponder = router.codexResponder
		}
		if selectedResponder == nil {
			return api.LocalProductConversationResponse{}, api.ErrLocalProductChatUnavailable
		}
		response, err := selectedResponder.Respond(ctx, request)
		if err == nil {
			return response, nil
		}
		code := "provider_unavailable"
		stage := "provider_connect"
		message := "Conversation Provider runtime is unavailable."
		retryable := true
		if errors.Is(err, context.DeadlineExceeded) {
			code = "timeout"
			message = "Provider connection timed out."
		} else if info, ok := api.LocalProductConversationDispatchFailureDetails(err); ok {
			// Preserve a specific failure raised by the responder (for example
			// "selected model has no verified Provider credential") so the App
			// can show an actionable message instead of a generic runtime error.
			code = info.Code
			stage = info.Stage
			message = info.UserMessage
			retryable = info.Retryable
		}
		return api.LocalProductConversationResponse{},
			api.NewLocalProductConversationDispatchErrorWithDetails(
				api.LocalProductConversationDispatchFailureInfo{
					Code: code, Stage: stage, UserMessage: message,
					Retryable: retryable,
				},
				errors.Join(api.ErrLocalProductChatUnavailable, err),
			)
	}
	selected, record, found := router.resolveBrokeredProfile(request.ProfileID)
	if !found {
		return api.LocalProductConversationResponse{}, api.ErrLocalProductChatUnavailable
	}
	wantBinding, bindingErr := router.ResolveConversationExecutionBinding(
		ctx, request.ProfileID,
	)
	if router.requireExecutionBinding &&
		(bindingErr != nil || request.ExecutionBinding == nil ||
			*request.ExecutionBinding != wantBinding) {
		return api.LocalProductConversationResponse{},
			api.NewLocalProductConversationDispatchError(
				"invalid_request", "conversation_dispatch", false,
				api.ErrLocalProductChatUnavailable,
			)
	}
	messages, err := productProviderConversationMessages(request.Messages)
	if err != nil {
		return api.LocalProductConversationResponse{}, err
	}
	modelID := strings.TrimSpace(request.ModelID)
	reasoningEffort := strings.TrimSpace(request.ReasoningEffort)
	if modelID != "" {
		if _, err := provider.ValidateConversationModel(
			record.ProviderID, modelID,
		); err != nil {
			return api.LocalProductConversationResponse{},
				api.NewLocalProductConversationDispatchError(
					"invalid_request", "conversation_dispatch", false,
					api.ErrLocalProductChatUnavailable,
				)
		}
	}
	var content string
	err = router.leasing.UseCredential(
		ctx,
		credentialvault.CredentialIdentity{
			ProviderID: record.ProviderID, ProviderAccountID: record.ProviderAccountID,
			CredentialReference: record.CredentialReference,
			CredentialRevision:  record.Revision,
		},
		func(leaseContext context.Context, secret []byte) error {
			var callErr error
			if modelID != "" || reasoningEffort != "" {
				content, callErr = selected.client.RespondConfigured(
					leaseContext, messages, secret, modelID, reasoningEffort,
				)
			} else {
				content, callErr = selected.client.Respond(leaseContext, messages, secret)
			}
			return callErr
		},
	)
	if err != nil {
		if stage := credentials.CredentialFailureStage(err); stage != "" {
			return api.LocalProductConversationResponse{}, api.NewLocalProductConversationDispatchErrorWithDetails(
				api.LocalProductConversationDispatchFailureInfo{
					Code:        "credential_unavailable",
					Stage:       stage,
					UserMessage: "Provider Account credential is unavailable.",
					Retryable:   credentials.CredentialFailureRetryable(stage),
				},
				errors.Join(api.ErrLocalProductChatUnavailable, err),
			)
		}
		if failure, ok := provider.ConversationFailureDetails(err); ok {
			return api.LocalProductConversationResponse{}, api.NewLocalProductConversationDispatchErrorWithDetails(
				api.LocalProductConversationDispatchFailureInfo{
					Code: failure.Code, Stage: failure.Stage,
					HTTPStatus: failure.HTTPStatus, ProviderCode: failure.ProviderCode,
					UserMessage: failure.UserMessage, Retryable: failure.Retryable,
					RetryAfterSeconds: failure.RetryAfterSeconds,
				},
				errors.Join(api.ErrLocalProductChatUnavailable, err),
			)
		}
		return api.LocalProductConversationResponse{}, api.ErrLocalProductChatUnavailable
	}
	return api.LocalProductConversationResponse{Content: content, Tentative: true}, nil
}

func productProviderConversationMessages(
	messages []api.LocalProductChatMessage,
) ([]provider.ConversationMessage, error) {
	const maximumBytes = 60 * 1024
	result := make([]provider.ConversationMessage, 0, 64)
	total := 0
	for index := len(messages) - 1; index >= 0 && len(result) < 64; index-- {
		message := messages[index]
		role := "assistant"
		if message.Role == string(api.ChatRoleUser) {
			role = "user"
		} else if message.Role != string(api.ChatRoleLoom) &&
			message.Role != string(api.ChatRoleProposal) &&
			message.Role != string(api.ChatRoleConfirmation) {
			return nil, api.ErrLocalProductChatUnavailable
		}
		content := message.DisplayContent()
		if content == "" || total+len(content) > maximumBytes {
			break
		}
		total += len(content)
		result = append(result, provider.ConversationMessage{Role: role, Content: content})
	}
	if len(result) == 0 || result[0].Role != "user" {
		return nil, api.ErrLocalProductChatUnavailable
	}
	for left, right := 0, len(result)-1; left < right; left, right = left+1, right-1 {
		result[left], result[right] = result[right], result[left]
	}
	return result, nil
}
