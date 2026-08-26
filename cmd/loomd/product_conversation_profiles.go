package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

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

const (
	productConversationCapacityCounterID      = "loom-unicode-rune-quarter-estimate"
	productConversationCapacityCounterVersion = "v1"
	productConversationAdapterOverheadTokens  = 1_024
)

type productConversationEstimatedTokenCounter struct{}

func (*productConversationEstimatedTokenCounter) ID() string {
	return productConversationCapacityCounterID
}

func (*productConversationEstimatedTokenCounter) Version() string {
	return productConversationCapacityCounterVersion
}

func (*productConversationEstimatedTokenCounter) CountTokens(content []byte) (int, error) {
	count := (utf8.RuneCount(content) + 3) / 4
	if count < 1 {
		return 1, nil
	}
	return count, nil
}

var productConversationCapacityCounter contextcapsule.TokenCounter = &productConversationEstimatedTokenCounter{}

func (router *productConversationProfileRouter) ResolveConversationContextCapacity(
	ctx context.Context,
	target contextcapsule.Target,
) (contextcapsule.CapacityAuthority, contextcapsule.TokenCounter, error) {
	if router == nil || ctx == nil || ctx.Err() != nil ||
		target.ProviderID == "" || target.ProviderID != strings.TrimSpace(target.ProviderID) ||
		target.ModelID == "" || target.ModelID != strings.TrimSpace(target.ModelID) ||
		target.TokenBudget <= 0 {
		return contextcapsule.CapacityAuthority{}, nil,
			api.ErrLocalProductChatUnavailable
	}
	authority := contextcapsule.CapacityAuthority{
		SchemaVersion:       contextcapsule.CapacitySchemaVersion,
		Status:              contextcapsule.CapacityUnavailable,
		TokenCounterID:      productConversationCapacityCounter.ID(),
		TokenCounterVersion: productConversationCapacityCounter.Version(),
	}
	window, reserved, known := productConversationModelCapacity(
		target.ProviderID, target.ModelID,
	)
	if known {
		// Vendor limits are known for this exact model key, while Loom's local
		// rune-based counter remains an estimate rather than a vendor tokenizer.
		authority.Status = contextcapsule.CapacityEstimated
		authority.ContextWindowTokens = window
		authority.ReservedOutputTokens = reserved
		authority.AdapterToolOverheadTokens =
			productConversationAdapterOverheadTokens
	}
	return authority, productConversationCapacityCounter, nil
}

func productConversationModelCapacity(
	providerID string,
	modelID string,
) (contextWindowTokens int, reservedOutputTokens int, known bool) {
	switch providerID + "\x00" + modelID {
	case "deepseek\x00deepseek-v4-flash", "deepseek\x00deepseek-v4-pro":
		return 1_000_000, 384_000, true
	case "anthropic\x00claude-sonnet-5":
		return 1_000_000, 128_000, true
	case "openai\x00gpt-5.6-sol", "openai\x00gpt-5.6-terra",
		"openai\x00gpt-5.6-luna", "openai\x00gpt-5.5",
		"openai\x00gpt-5.4":
		return 1_050_000, 128_000, true
	case "openai\x00gpt-5.4-mini":
		return 400_000, 128_000, true
	default:
		return 0, 0, false
	}
}

func (router *productConversationProfileRouter) ResolveConversationContextTarget(
	ctx context.Context,
	threadID string,
	segmentID string,
	profileID string,
	modelID string,
) (contextcapsule.Target, error) {
	if router == nil || ctx == nil || ctx.Err() != nil || threadID == "" ||
		segmentID == "" {
		return contextcapsule.Target{}, api.ErrLocalProductChatUnavailable
	}
	if profileID == provider.PiConversationProfileID {
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
	if profileID == provider.CodexConversationProfileID {
		resolvedModelID, ok := resolveCodexConversationModel(modelID)
		if !ok {
			return contextcapsule.Target{}, api.ErrLocalProductChatUnavailable
		}
		return contextcapsule.Target{
			ConversationID: threadID, TeamID: "conversation:" + threadID,
			AgentID: "conversation-agent", RoleID: segmentID,
			ProviderID: "openai", ModelID: resolvedModelID, AuthMode: "native_auth",
			ContextAdapterID:        "context:codex:v1",
			DisclosurePolicyID:      "loom.local-conversation-disclosure",
			DisclosurePolicyVersion: 1, TokenBudget: 8_192,
		}, nil
	}
	if profileID == provider.ClaudeCodeConversationProfileID {
		if modelID != provider.AnthropicConversationModelID {
			return contextcapsule.Target{}, api.ErrLocalProductChatUnavailable
		}
		return contextcapsule.Target{
			ConversationID: threadID, TeamID: "conversation:" + threadID,
			AgentID: "conversation-agent", RoleID: segmentID,
			ProviderID: "anthropic", ModelID: provider.AnthropicConversationModelID,
			AuthMode: "native_auth", ContextAdapterID: "context:claude-code:v1",
			DisclosurePolicyID:      "loom.local-conversation-disclosure",
			DisclosurePolicyVersion: 1, TokenBudget: 8_192,
		}, nil
	}
	if profileID == provider.OpenCodeConversationProfileID {
		if !validOpenCodeNativeConversationModel(modelID) {
			return contextcapsule.Target{}, api.ErrLocalProductChatUnavailable
		}
		return contextcapsule.Target{
			ConversationID: threadID, TeamID: "conversation:" + threadID,
			AgentID: "conversation-agent", RoleID: segmentID,
			ProviderID: "opencode", ModelID: modelID,
			AuthMode: "native_auth", ContextAdapterID: "context:loom-native:v1",
			DisclosurePolicyID:      "loom.local-conversation-disclosure",
			DisclosurePolicyVersion: 1, TokenBudget: 8_192,
		}, nil
	}
	if record, found := router.resolveOpenCodeProfile(profileID); found {
		if !openCodeModelMatchesProvider(modelID, record.ProviderID) {
			return contextcapsule.Target{}, api.ErrLocalProductChatUnavailable
		}
		disclosurePolicyID := "loom.local-conversation-disclosure"
		disclosurePolicyVersion := 1
		if policy, ok := router.policy(record.ProviderID, record.ProviderAccountID); ok {
			disclosurePolicyID = fmt.Sprintf("provider-account-policy:%s", policy.Digest())
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
	selected, record, found := router.resolveBrokeredProfile(profileID)
	if !found {
		return contextcapsule.Target{}, api.ErrLocalProductChatUnavailable
	}
	targetModelID := strings.TrimSpace(modelID)
	if targetModelID == "" {
		targetModelID = productConversationModelID(selected.providerID)
	}
	if targetModelID == "" {
		return contextcapsule.Target{}, api.ErrLocalProductChatUnavailable
	}
	if _, err := provider.ValidateConversationModel(selected.providerID, targetModelID); err != nil {
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
		ModelID: targetModelID, AuthMode: record.AuthMode,
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
	openCodeResponder       api.LocalProductConversationResponder
	codexResponder          api.LocalProductConversationResponder
	claudeCodeResponder     api.LocalProductConversationResponder
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
		leasing, deepSeek, false, nil, nil, additionalRoutes...,
	)
}

func newProductConversationProfileRouterWithPolicy(
	defaultResponder api.LocalProductConversationResponder,
	credential productConversationCredentialLookup,
	policy productConversationPolicyLookup,
	leasing productCredentialLeaseAccess,
	deepSeek productDeepSeekConversationClient,
	requireExecutionBinding bool,
	openCodeResponder api.LocalProductConversationResponder,
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
		openCodeResponder:       openCodeResponder,
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
	modelID string,
) (api.LocalProductConversationExecutionBinding, error) {
	if router == nil || profileID == "" {
		return api.LocalProductConversationExecutionBinding{},
			api.ErrLocalProductChatUnavailable
	}
	if profileID == provider.PiConversationProfileID {
		if _, localPi := router.defaultResponder.(*productPiConversationResponder); !localPi {
			return api.LocalProductConversationExecutionBinding{},
				api.ErrLocalProductChatUnavailable
		}
		return api.LocalProductConversationExecutionBinding{
			SchemaVersion: 4, HarnessAdapter: "pi", ProviderID: "loom-local",
			ModelID: "qwen2.5-coder-1.5b-instruct-q4-k-m",
		}, nil
	}
	if profileID == provider.CodexConversationProfileID {
		resolvedModelID, ok := resolveCodexConversationModel(modelID)
		if !ok {
			return api.LocalProductConversationExecutionBinding{}, api.ErrLocalProductChatUnavailable
		}
		return api.LocalProductConversationExecutionBinding{
			SchemaVersion: 4, HarnessAdapter: "codex", ProviderID: "openai",
			ModelID: resolvedModelID,
		}, nil
	}
	if profileID == provider.ClaudeCodeConversationProfileID {
		if modelID != provider.AnthropicConversationModelID {
			return api.LocalProductConversationExecutionBinding{},
				api.ErrLocalProductChatUnavailable
		}
		return api.LocalProductConversationExecutionBinding{
			SchemaVersion: 4, HarnessAdapter: "claude-code",
			ProviderID: "anthropic", ModelID: provider.AnthropicConversationModelID,
		}, nil
	}
	if profileID == provider.OpenCodeConversationProfileID {
		if !validOpenCodeNativeConversationModel(modelID) {
			return api.LocalProductConversationExecutionBinding{}, api.ErrLocalProductChatUnavailable
		}
		return api.LocalProductConversationExecutionBinding{
			SchemaVersion: 4, HarnessAdapter: "opencode", ProviderID: "opencode",
			ModelID: modelID,
		}, nil
	}
	if record, found := router.resolveOpenCodeProfile(profileID); found {
		if !openCodeModelMatchesProvider(modelID, record.ProviderID) {
			return api.LocalProductConversationExecutionBinding{}, api.ErrLocalProductChatUnavailable
		}
		binding := api.LocalProductConversationExecutionBinding{
			SchemaVersion: 4, HarnessAdapter: "opencode",
			ProviderID: record.ProviderID, ProviderAccountID: record.ProviderAccountID,
			CredentialRevision: record.Revision, ModelID: modelID,
		}
		applyConversationAccountPolicy(router, &binding, record)
		return binding, nil
	}
	_, record, found := router.resolveBrokeredProfile(profileID)
	if !found {
		return api.LocalProductConversationExecutionBinding{},
			api.ErrLocalProductChatUnavailable
	}
	binding := api.LocalProductConversationExecutionBinding{
		SchemaVersion: 4, HarnessAdapter: "loom-native", ProviderID: record.ProviderID,
		ProviderAccountID:  record.ProviderAccountID,
		CredentialRevision: record.Revision,
	}
	binding.ModelID = strings.TrimSpace(modelID)
	if binding.ModelID == "" {
		binding.ModelID = productConversationModelID(record.ProviderID)
	}
	if _, err := provider.ValidateConversationModel(record.ProviderID, binding.ModelID); err != nil {
		return api.LocalProductConversationExecutionBinding{}, api.ErrLocalProductChatUnavailable
	}
	applyConversationAccountPolicy(router, &binding, record)
	return binding, nil
}

func resolveCodexConversationModel(modelID string) (string, bool) {
	modelID = strings.TrimSpace(modelID)
	if modelID == "" {
		modelID = "codex-default"
	}
	_, err := provider.ValidateConversationModel("openai", modelID)
	return modelID, err == nil
}

func applyConversationAccountPolicy(
	router *productConversationProfileRouter,
	binding *api.LocalProductConversationExecutionBinding,
	record projection.ProviderCredentialRecord,
) {
	if router == nil || binding == nil {
		return
	}
	if policy, ok := router.policy(record.ProviderID, record.ProviderAccountID); ok {
		binding.ProviderAccountPolicyVersion = policy.Version()
		binding.ProviderAccountPolicyRevision = policy.Revision()
		binding.ProviderAccountPolicyDigest = policy.Digest()
		binding.TrustDomain = policy.TrustDomain()
		binding.RetentionMode = policy.RetentionMode()
		binding.DataRegion = policy.DataRegion()
	}
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

func (router *productConversationProfileRouter) resolveOpenCodeProfile(
	profileID string,
) (projection.ProviderCredentialRecord, bool) {
	if router == nil || profileID == "" || router.credential == nil {
		return projection.ProviderCredentialRecord{}, false
	}
	var selected projection.ProviderCredentialRecord
	found := false
	for _, descriptor := range provider.Catalog() {
		runtimeProviderID, ok := provider.OpenCodeRuntimeProviderID(descriptor.ID)
		if !ok {
			continue
		}
		if _, known := provider.OpenCodeCredentialEnv(runtimeProviderID); !known {
			continue
		}
		for _, candidate := range router.credential(descriptor.ID) {
			if profileID != provider.OpenCodeConversationAccountProfileID(
				candidate.ProviderID, candidate.ProviderAccountID, candidate.Revision,
			) {
				continue
			}
			if found || candidate.ProviderID != descriptor.ID ||
				!credentials.ValidProviderAccountIdentifier(
					candidate.ProviderID, candidate.ProviderAccountID,
				) || candidate.AuthMode != "brokered" ||
				candidate.Status != string(credentials.CredentialVerified) ||
				candidate.Reason != "" || candidate.CredentialReference == "" ||
				candidate.Revision <= 0 {
				return projection.ProviderCredentialRecord{}, false
			}
			selected, found = candidate, true
		}
	}
	return selected, found
}

func validOpenCodeNativeConversationModel(modelID string) bool {
	runtimeProviderID, ok := openCodeModelProviderID(strings.TrimSpace(modelID))
	return ok && runtimeProviderID == "opencode"
}

func openCodeModelMatchesProvider(modelID, loomProviderID string) bool {
	runtimeProviderID, ok := openCodeModelProviderID(strings.TrimSpace(modelID))
	if !ok {
		return false
	}
	boundProviderID, mapped := provider.OpenCodeLoomProviderID(runtimeProviderID)
	if !mapped {
		boundProviderID = runtimeProviderID
	}
	return boundProviderID == loomProviderID
}

func (router *productConversationProfileRouter) Respond(
	ctx context.Context,
	request api.LocalProductConversationRequest,
) (api.LocalProductConversationResponse, error) {
	if router == nil || ctx == nil || len(request.Messages) == 0 ||
		request.ProfileID == "" {
		return api.LocalProductConversationResponse{}, api.ErrLocalProductChatUnavailable
	}
	if request.ProfileID == provider.PiConversationProfileID ||
		request.ProfileID == provider.CodexConversationProfileID ||
		request.ProfileID == provider.ClaudeCodeConversationProfileID ||
		request.ProfileID == provider.OpenCodeConversationProfileID {
		if router.requireExecutionBinding {
			wantBinding, bindingErr := router.ResolveConversationExecutionBinding(
				ctx, request.ProfileID, strings.TrimSpace(request.ModelID),
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
		// Native harness profiles retain dedicated responders even when Pi is
		// installed as the default conversation runtime.
		selectedResponder := router.defaultResponder
		if request.ProfileID == provider.PiConversationProfileID {
			if _, localPi := router.defaultResponder.(*productPiConversationResponder); !localPi {
				return api.LocalProductConversationResponse{}, api.ErrLocalProductChatUnavailable
			}
		}
		if request.ProfileID == provider.CodexConversationProfileID &&
			router.codexResponder != nil {
			selectedResponder = router.codexResponder
		}
		if request.ProfileID == provider.ClaudeCodeConversationProfileID {
			selectedResponder = router.claudeCodeResponder
		}
		if request.ProfileID == provider.OpenCodeConversationProfileID {
			selectedResponder = router.openCodeResponder
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
	if record, found := router.resolveOpenCodeProfile(request.ProfileID); found {
		wantBinding, bindingErr := router.ResolveConversationExecutionBinding(
			ctx, request.ProfileID, strings.TrimSpace(request.ModelID),
		)
		if bindingErr != nil || router.requireExecutionBinding &&
			(request.ExecutionBinding == nil || *request.ExecutionBinding != wantBinding) {
			return api.LocalProductConversationResponse{},
				api.NewLocalProductConversationDispatchError(
					"invalid_request", "conversation_dispatch", false,
					api.ErrLocalProductChatUnavailable,
				)
		}
		boundResponder, ok := router.openCodeResponder.(interface {
			RespondBound(
				context.Context,
				api.LocalProductConversationRequest,
				credentialvault.CredentialIdentity,
			) (api.LocalProductConversationResponse, error)
		})
		if !ok {
			return api.LocalProductConversationResponse{}, api.ErrLocalProductChatUnavailable
		}
		response, err := boundResponder.RespondBound(
			ctx, request,
			credentialvault.CredentialIdentity{
				ProviderID: record.ProviderID, ProviderAccountID: record.ProviderAccountID,
				CredentialReference: record.CredentialReference,
				CredentialRevision:  record.Revision,
			},
		)
		if err == nil {
			return response, nil
		}
		if info, specific := api.LocalProductConversationDispatchFailureDetails(err); specific {
			return api.LocalProductConversationResponse{},
				api.NewLocalProductConversationDispatchErrorWithDetails(
					info, errors.Join(api.ErrLocalProductChatUnavailable, err),
				)
		}
		return api.LocalProductConversationResponse{}, api.ErrLocalProductChatUnavailable
	}
	selected, record, found := router.resolveBrokeredProfile(request.ProfileID)
	if !found {
		return api.LocalProductConversationResponse{}, api.ErrLocalProductChatUnavailable
	}
	wantBinding, bindingErr := router.ResolveConversationExecutionBinding(
		ctx, request.ProfileID, strings.TrimSpace(request.ModelID),
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
	messages, err := productProviderConversationMessages(
		request.Messages, request.ContextPrompt,
	)
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
	contextPrompt string,
) ([]provider.ConversationMessage, error) {
	const maximumBytes = 60 * 1024
	result := make([]provider.ConversationMessage, 0, 64)
	total := len(contextPrompt)
	if total > maximumBytes {
		return nil, api.ErrLocalProductChatUnavailable
	}
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
	if contextPrompt = strings.TrimSpace(contextPrompt); contextPrompt != "" {
		result = append([]provider.ConversationMessage{{
			Role: "system", Content: contextPrompt,
		}}, result...)
	}
	return result, nil
}
