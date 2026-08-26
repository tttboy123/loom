package main

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/credentials"
	credentialvault "loom-pi-rebuild/internal/credentials/vault"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/provider"
	"loom-pi-rebuild/internal/work"
)

type profileConversationStore struct {
	secret   []byte
	lastRead []byte
	reads    int
}

func (store *profileConversationStore) Put(context.Context, string, []byte) error {
	return errors.New("unexpected put")
}

func (store *profileConversationStore) Read(
	_ context.Context,
	_ string,
) ([]byte, error) {
	store.reads++
	store.lastRead = append([]byte(nil), store.secret...)
	return store.lastRead, nil
}

func (store *profileConversationStore) Delete(context.Context, string) error {
	return errors.New("unexpected delete")
}

type profileDeepSeekClient struct {
	messages        []provider.ConversationMessage
	secret          []byte
	modelID         string
	reasoningEffort string
	reply           string
}

type profileConversationResponderFunc func(
	context.Context,
	api.LocalProductConversationRequest,
) (api.LocalProductConversationResponse, error)

func (respond profileConversationResponderFunc) Respond(
	ctx context.Context,
	request api.LocalProductConversationRequest,
) (api.LocalProductConversationResponse, error) {
	return respond(ctx, request)
}

type profileConversationLeaseFailure struct{ stage string }

func (failure profileConversationLeaseFailure) UseCredential(
	context.Context,
	credentialvault.CredentialIdentity,
	func(context.Context, []byte) error,
) error {
	return credentials.WithCredentialFailureStage(
		failure.stageOrDefault(),
		credentials.ErrCredentialStoreUnavailable,
	)
}

func (failure profileConversationLeaseFailure) stageOrDefault() string {
	if failure.stage == "" {
		return credentials.CredentialStageLeaseIssue
	}
	return failure.stage
}

type profileConversationLeaseRecorder struct {
	identity credentialvault.CredentialIdentity
	calls    int
}

func (recorder *profileConversationLeaseRecorder) UseCredential(
	ctx context.Context,
	identity credentialvault.CredentialIdentity,
	use func(context.Context, []byte) error,
) error {
	recorder.calls++
	recorder.identity = identity
	secret := []byte("account-private-key")
	defer clearProductCredentialLeaseSecret(secret)
	return use(ctx, secret)
}

func (client *profileDeepSeekClient) Respond(
	_ context.Context,
	messages []provider.ConversationMessage,
	secret []byte,
) (string, error) {
	return client.RespondConfigured(
		context.Background(), messages, secret, "deepseek-chat", "",
	)
}

func (client *profileDeepSeekClient) RespondConfigured(
	_ context.Context,
	messages []provider.ConversationMessage,
	secret []byte,
	modelID string,
	reasoningEffort string,
) (string, error) {
	client.messages = append([]provider.ConversationMessage(nil), messages...)
	client.secret = append([]byte(nil), secret...)
	client.modelID = modelID
	client.reasoningEffort = reasoningEffort
	if client.reply != "" {
		return client.reply, nil
	}
	return "DeepSeek reply", nil
}

func TestConversationProfileRouterUsesExactVerifiedDeepSeekRevision(t *testing.T) {
	store := &profileConversationStore{secret: []byte("private-key")}
	client := &profileDeepSeekClient{}
	router, err := newProductConversationProfileRouter(
		nil,
		func(string) []projection.ProviderCredentialRecord {
			return []projection.ProviderCredentialRecord{{
				ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
				CredentialReference: "credential-ref-deepseek-1",
				Revision:            2, Status: "verified", AuthMode: "brokered",
			}}
		},
		profileConversationLeasing(t, store),
		client,
	)
	if err != nil {
		t.Fatal(err)
	}
	response, err := router.Respond(context.Background(), api.LocalProductConversationRequest{
		ThreadID:  "thread-profile",
		ProfileID: provider.DeepSeekConversationProfileID(2),
		Messages: []api.LocalProductChatMessage{
			{Role: "user", Content: "hello"},
			{Role: "loom", Content: "prior reply"},
			{Role: "user", Content: "continue"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if response.Content != "DeepSeek reply" || !response.Tentative ||
		store.reads != 1 || string(client.secret) != "private-key" ||
		len(client.messages) != 3 || client.messages[1].Role != "assistant" {
		t.Fatalf("response=%#v reads=%d client=%#v", response, store.reads, client)
	}
	for _, value := range store.lastRead {
		if value != 0 {
			t.Fatalf("credential lease buffer was not cleared: %v", store.lastRead)
		}
	}
	if _, err := router.Respond(context.Background(), api.LocalProductConversationRequest{
		ThreadID:  "thread-profile",
		ProfileID: provider.DeepSeekConversationProfileID(3),
		Messages:  []api.LocalProductChatMessage{{Role: "user", Content: "wrong revision"}},
	}); err == nil || store.reads != 1 {
		t.Fatalf("stale profile error=%v reads=%d", err, store.reads)
	}
}

func TestConversationProfileRouterUsesExactSameProviderAccountRevision(t *testing.T) {
	leasing := &profileConversationLeaseRecorder{}
	records := []projection.ProviderCredentialRecord{
		{
			ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
			CredentialReference: "credential-ref-deepseek-primary",
			Revision:            7, Status: "verified", AuthMode: "brokered",
		},
		{
			ProviderID: "deepseek", ProviderAccountID: "deepseek.secondary",
			CredentialReference: "credential-ref-deepseek-secondary",
			Revision:            7, Status: "verified", AuthMode: "brokered",
		},
	}
	router, err := newProductConversationProfileRouter(
		nil,
		func(providerID string) []projection.ProviderCredentialRecord {
			if providerID != "deepseek" {
				return nil
			}
			return records
		},
		leasing,
		&profileDeepSeekClient{},
	)
	if err != nil {
		t.Fatal(err)
	}
	profileID := provider.DeepSeekConversationAccountProfileID("deepseek.secondary", 7)
	response, err := router.Respond(
		context.Background(), api.LocalProductConversationRequest{
			ThreadID: "thread-secondary", ProfileID: profileID,
			Messages: []api.LocalProductChatMessage{{Role: "user", Content: "hello"}},
		},
	)
	if err != nil || response.Content != "DeepSeek reply" || leasing.calls != 1 ||
		leasing.identity.ProviderID != "deepseek" ||
		leasing.identity.ProviderAccountID != "deepseek.secondary" ||
		leasing.identity.CredentialReference != "credential-ref-deepseek-secondary" ||
		leasing.identity.CredentialRevision != 7 {
		t.Fatalf(
			"response=%#v error=%v lease=%#v calls=%d",
			response, err, leasing.identity, leasing.calls,
		)
	}
}

func TestConversationContextTargetFreezesExactDeepSeekAccountModelAndPolicy(t *testing.T) {
	policy, err := work.NewProviderAccountPolicy(work.ProviderAccountPolicyInput{
		Version: 2, ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
		Revision: 4, MaximumConcurrentAttempts: 2, DispatchWindow: time.Hour,
		MaximumDispatchStarts: 10, MaximumAssignedBudgetUnits: 1_000,
		TrustDomain: "external_provider", RetentionMode: "zero_data_retention",
		DataRegion: "apac", ConfiguredAt: time.Date(2026, 8, 12, 1, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
	router, err := newProductConversationProfileRouterWithPolicy(
		nil,
		func(string) []projection.ProviderCredentialRecord {
			return []projection.ProviderCredentialRecord{{
				ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
				CredentialReference: "credential-ref-deepseek-work",
				Revision:            7, Status: "verified", AuthMode: "brokered",
			}}
		},
		func(providerID, accountID string) (work.ProviderAccountPolicy, bool) {
			return policy, providerID == "deepseek" && accountID == "deepseek.work"
		},
		&profileConversationLeaseRecorder{}, &profileDeepSeekClient{}, true, nil, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	target, err := router.ResolveConversationContextTarget(
		context.Background(), "conversation-one", "segment-2",
		provider.DeepSeekConversationAccountProfileID("deepseek.work", 7),
		provider.DeepSeekConversationModelID,
	)
	if err != nil || target.ConversationID != "conversation-one" ||
		target.RoleID != "segment-2" || target.ProviderID != "deepseek" ||
		target.ProviderAccountID != "deepseek.work" ||
		target.ModelID != provider.DeepSeekConversationModelID ||
		target.AuthMode != "brokered" ||
		target.ContextAdapterID != "context:loom-native:v1" ||
		target.DisclosurePolicyID != "provider-account-policy:"+policy.Digest() ||
		target.DisclosurePolicyVersion != policy.Version() {
		t.Fatalf("target=%#v error=%v", target, err)
	}
	if _, err := router.ResolveConversationContextTarget(
		context.Background(), "conversation-one", "segment-2",
		provider.DeepSeekConversationAccountProfileID("deepseek.primary", 7),
		provider.DeepSeekConversationModelID,
	); !errors.Is(err, api.ErrLocalProductChatUnavailable) {
		t.Fatalf("wrong account profile error = %v", err)
	}
}

func TestConversationProfileRouterDispatchesIdentityQuestionsToProvider(t *testing.T) {
	client := &profileDeepSeekClient{}
	leasing := &profileConversationLeaseRecorder{}
	router, err := newProductConversationProfileRouter(
		nil,
		func(string) []projection.ProviderCredentialRecord {
			return []projection.ProviderCredentialRecord{{
				ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
				CredentialReference: "credential-ref-deepseek-identity",
				Revision:            6, Status: "verified", AuthMode: "brokered",
			}}
		},
		leasing,
		client,
	)
	if err != nil {
		t.Fatal(err)
	}
	response, err := router.Respond(
		context.Background(),
		api.LocalProductConversationRequest{
			ThreadID:  "thread-route-identity",
			ProfileID: provider.DeepSeekConversationProfileID(6),
			Messages: []api.LocalProductChatMessage{
				{Role: "user", Content: "你的模型是谁"},
				{Role: "loom", Content: "我是 OpenAI GPT。", Tentative: true},
				{Role: "user", Content: "你是哪个底层模型"},
			},
		},
	)
	if err != nil || !response.Tentative || response.Content != "DeepSeek reply" ||
		leasing.calls != 1 || len(client.messages) != 3 ||
		client.messages[2].Content != "你是哪个底层模型" {
		t.Fatalf(
			"response=%#v error=%v lease_calls=%d client=%#v",
			response, err, leasing.calls, client,
		)
	}
	response, err = router.Respond(
		context.Background(),
		api.LocalProductConversationRequest{
			ThreadID:  "thread-route-persona",
			ProfileID: provider.DeepSeekConversationProfileID(6),
			Messages: []api.LocalProductChatMessage{
				{Role: "user", Content: "你是谁"},
			},
		},
	)
	if err != nil || !response.Tentative || response.Content != "DeepSeek reply" ||
		leasing.calls != 2 || len(client.messages) != 1 ||
		client.messages[0].Content != "你是谁" {
		t.Fatalf(
			"persona response=%#v error=%v lease_calls=%d client=%#v",
			response, err, leasing.calls, client,
		)
	}
}

func TestConversationProfileRouterUsesExactProviderAccountLease(t *testing.T) {
	client := &profileDeepSeekClient{}
	leasing := &profileConversationLeaseRecorder{}
	records := []projection.ProviderCredentialRecord{
		{
			ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
			CredentialReference: "credential-ref-deepseek-primary",
			Revision:            7, Status: "verified", AuthMode: "brokered",
		},
		{
			ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
			CredentialReference: "credential-ref-deepseek-work",
			Revision:            7, Status: "verified", AuthMode: "brokered",
		},
	}
	router, err := newProductConversationProfileRouter(
		nil,
		func(providerID string) []projection.ProviderCredentialRecord {
			if providerID != "deepseek" {
				return nil
			}
			return append([]projection.ProviderCredentialRecord(nil), records...)
		},
		leasing,
		client,
	)
	if err != nil {
		t.Fatal(err)
	}
	response, err := router.Respond(
		context.Background(), api.LocalProductConversationRequest{
			ThreadID: "thread-deepseek-work",
			ProfileID: provider.DeepSeekConversationAccountProfileID(
				"deepseek.work", 7,
			),
			Messages: []api.LocalProductChatMessage{{Role: "user", Content: "hello"}},
		},
	)
	if err != nil || response.Content != "DeepSeek reply" || leasing.calls != 1 ||
		leasing.identity.ProviderID != "deepseek" ||
		leasing.identity.ProviderAccountID != "deepseek.work" ||
		leasing.identity.CredentialReference != "credential-ref-deepseek-work" ||
		leasing.identity.CredentialRevision != 7 {
		t.Fatalf(
			"response=%#v error=%v lease=%#v calls=%d",
			response, err, leasing.identity, leasing.calls,
		)
	}
}

func TestConversationProfileRouterFreezesAndRevalidatesExactAccountPolicy(t *testing.T) {
	configuredAt := time.Date(2026, 8, 12, 10, 0, 0, 0, time.UTC)
	policy, err := work.NewProviderAccountPolicy(work.ProviderAccountPolicyInput{
		Version: 2, ProviderID: "deepseek",
		ProviderAccountID: "deepseek.work", Revision: 4,
		MaximumConcurrentAttempts: 2, DispatchWindow: time.Minute,
		MaximumDispatchStarts: 20, MaximumAssignedBudgetUnits: 100,
		TrustDomain:   "enterprise_tenant",
		RetentionMode: "zero_data_retention", DataRegion: "apac",
		ConfiguredAt: configuredAt,
	})
	if err != nil {
		t.Fatal(err)
	}
	current := policy
	client := &profileDeepSeekClient{}
	leasing := &profileConversationLeaseRecorder{}
	profileID := provider.DeepSeekConversationAccountProfileID("deepseek.work", 7)
	router, err := newProductConversationProfileRouterWithPolicy(
		nil,
		func(string) []projection.ProviderCredentialRecord {
			return []projection.ProviderCredentialRecord{{
				ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
				CredentialReference: "credential-ref-deepseek-work",
				Revision:            7, Status: "verified", AuthMode: "brokered",
			}}
		},
		func(providerID, accountID string) (work.ProviderAccountPolicy, bool) {
			return current, current.ProviderID() == providerID &&
				current.ProviderAccountID() == accountID
		},
		leasing, client, true, nil, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := router.ResolveConversationExecutionBinding(
		context.Background(), profileID, provider.DeepSeekConversationModelID,
	)
	if err != nil || binding.ProviderAccountPolicyVersion != 2 ||
		binding.ProviderAccountPolicyRevision != 4 ||
		binding.ProviderAccountPolicyDigest != policy.Digest() ||
		binding.TrustDomain != "enterprise_tenant" ||
		binding.RetentionMode != "zero_data_retention" ||
		binding.DataRegion != "apac" {
		t.Fatalf("binding=%#v error=%v", binding, err)
	}
	current, err = work.NewProviderAccountPolicy(work.ProviderAccountPolicyInput{
		Version: 2, ProviderID: "deepseek",
		ProviderAccountID: "deepseek.work", Revision: 5,
		MaximumConcurrentAttempts: 2, DispatchWindow: time.Minute,
		MaximumDispatchStarts: 20, MaximumAssignedBudgetUnits: 100,
		TrustDomain:   "external_provider",
		RetentionMode: "provider_default", DataRegion: "global",
		ConfiguredAt: configuredAt.Add(time.Minute),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = router.Respond(context.Background(), api.LocalProductConversationRequest{
		ThreadID: "thread-policy", ProfileID: profileID,
		ExecutionBinding: &binding,
		Messages:         []api.LocalProductChatMessage{{Role: "user", Content: "hello"}},
	})
	if code, stage, retryable, ok := api.LocalProductConversationDispatchFailure(err); !ok || code != "invalid_request" || stage != "conversation_dispatch" ||
		retryable || leasing.calls != 0 {
		t.Fatalf("error=%v code=%s stage=%s retryable=%v calls=%d", err, code, stage, retryable, leasing.calls)
	}
}

func TestConversationProfileRouterUsesExactAnthropicAccountLease(t *testing.T) {
	client := &profileDeepSeekClient{reply: "Anthropic reply"}
	leasing := &profileConversationLeaseRecorder{}
	router, err := newProductConversationProfileRouter(
		nil,
		func(providerID string) []projection.ProviderCredentialRecord {
			if providerID != "anthropic" {
				return nil
			}
			return []projection.ProviderCredentialRecord{{
				ProviderID: "anthropic", ProviderAccountID: "anthropic.work",
				CredentialReference: "credential-ref-anthropic-work",
				Revision:            5, Status: "verified", AuthMode: "brokered",
			}}
		},
		leasing,
		&profileDeepSeekClient{},
		productConversationProviderRoute{
			providerID: "anthropic",
			profileID:  provider.AnthropicConversationAccountProfileID,
			client:     client,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	response, err := router.Respond(
		context.Background(),
		api.LocalProductConversationRequest{
			ThreadID: "thread-anthropic-work",
			ProfileID: provider.AnthropicConversationAccountProfileID(
				"anthropic.work", 5,
			),
			Messages: []api.LocalProductChatMessage{{Role: "user", Content: "hello"}},
		},
	)
	if err != nil || response.Content != "Anthropic reply" || leasing.calls != 1 ||
		leasing.identity.ProviderID != "anthropic" ||
		leasing.identity.ProviderAccountID != "anthropic.work" ||
		leasing.identity.CredentialReference != "credential-ref-anthropic-work" ||
		leasing.identity.CredentialRevision != 5 {
		t.Fatalf(
			"response=%#v error=%v lease=%#v calls=%d",
			response, err, leasing.identity, leasing.calls,
		)
	}
}

func TestConversationProfileRouterPreservesVaultLeaseFailureStage(t *testing.T) {
	router, err := newProductConversationProfileRouter(
		nil,
		func(string) []projection.ProviderCredentialRecord {
			return []projection.ProviderCredentialRecord{{
				ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
				CredentialReference: "credential-ref-deepseek-lease",
				Revision:            2, Status: "verified", AuthMode: "brokered",
			}}
		},
		profileConversationLeaseFailure{},
		&profileDeepSeekClient{},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = router.Respond(
		context.Background(), api.LocalProductConversationRequest{
			ThreadID:  "thread-vault-lease",
			ProfileID: provider.DeepSeekConversationProfileID(2),
			Messages:  []api.LocalProductChatMessage{{Role: "user", Content: "hello"}},
		},
	)
	if !errors.Is(err, api.ErrLocalProductChatUnavailable) ||
		credentials.CredentialFailureStage(err) != credentials.CredentialStageLeaseIssue {
		t.Fatalf("router error = %v stage=%q", err, credentials.CredentialFailureStage(err))
	}
	failure, ok := api.LocalProductConversationDispatchFailureDetails(err)
	if !ok || failure.Code != "credential_unavailable" ||
		failure.Stage != credentials.CredentialStageLeaseIssue ||
		!failure.Retryable {
		t.Fatalf("failure = %#v, ok=%t, error=%v", failure, ok, err)
	}
	response := productConversationServiceError(err)
	if response.Error == nil || response.Error.Code != "credential_unavailable" ||
		response.Error.Stage != credentials.CredentialStageLeaseIssue ||
		!response.Error.Recoverable {
		t.Fatalf("IPC error = %#v", response.Error)
	}
}

func TestConversationProfileRouterVaultRecoveryFailureIsNotRetryable(t *testing.T) {
	router, err := newProductConversationProfileRouter(
		nil,
		func(string) []projection.ProviderCredentialRecord {
			return []projection.ProviderCredentialRecord{{
				ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
				CredentialReference: "credential-ref-deepseek-corrupt",
				Revision:            2, Status: "verified", AuthMode: "brokered",
			}}
		},
		profileConversationLeaseFailure{stage: credentials.CredentialStageVaultDecrypt},
		&profileDeepSeekClient{},
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = router.Respond(
		context.Background(), api.LocalProductConversationRequest{
			ThreadID:  "thread-vault-decrypt",
			ProfileID: provider.DeepSeekConversationProfileID(2),
			Messages:  []api.LocalProductChatMessage{{Role: "user", Content: "hello"}},
		},
	)
	failure, ok := api.LocalProductConversationDispatchFailureDetails(err)
	if !ok || failure.Code != "credential_unavailable" ||
		failure.Stage != credentials.CredentialStageVaultDecrypt ||
		failure.Retryable {
		t.Fatalf("failure = %#v, ok=%t, error=%v", failure, ok, err)
	}
	response := productConversationServiceError(err)
	if response.Error == nil || response.Error.Code != "credential_unavailable" ||
		response.Error.Stage != credentials.CredentialStageVaultDecrypt ||
		response.Error.Recoverable {
		t.Fatalf("IPC error = %#v", response.Error)
	}
}

func TestConversationProfileRouterPreservesSafeProviderHTTPFailure(t *testing.T) {
	client, err := provider.NewDeepSeekConversationClient(
		provider.DeepSeekConversationConfig{
			Client: profileConversationHTTPDoerFunc(func(request *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusPaymentRequired,
					Body: io.NopCloser(strings.NewReader(
						`{"error":{"code":"insufficient_balance","message":"must never escape"}}`,
					)),
					Request: request,
				}, nil
			}),
			Timeout: time.Second, MaxResponseBytes: 4096,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	router, err := newProductConversationProfileRouter(
		nil,
		func(string) []projection.ProviderCredentialRecord {
			return []projection.ProviderCredentialRecord{{
				ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
				CredentialReference: "credential-ref-deepseek-http",
				Revision:            6, Status: "verified", AuthMode: "brokered",
			}}
		},
		profileConversationLeasing(
			t,
			&profileConversationStore{secret: []byte("private-key")},
		),
		client,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = router.Respond(context.Background(), api.LocalProductConversationRequest{
		ThreadID:  "thread-provider-http",
		ProfileID: provider.DeepSeekConversationProfileID(6),
		Messages:  []api.LocalProductChatMessage{{Role: "user", Content: "hello"}},
	})
	failure, ok := api.LocalProductConversationDispatchFailureDetails(err)
	if !ok || failure.Code != "provider_insufficient_balance" ||
		failure.Stage != "provider_http" || failure.HTTPStatus != 402 ||
		failure.ProviderCode != "insufficient_balance" || failure.Retryable ||
		failure.UserMessage != "Provider account has insufficient balance." ||
		strings.Contains(err.Error(), "must never escape") {
		t.Fatalf("failure=%#v ok=%v error=%v", failure, ok, err)
	}
}

type profileConversationHTTPDoerFunc func(*http.Request) (*http.Response, error)

func (doer profileConversationHTTPDoerFunc) Do(
	request *http.Request,
) (*http.Response, error) {
	return doer(request)
}

func TestConversationProfileRouterUsesExactKimiAndMiniMaxRevision(t *testing.T) {
	store := &profileConversationStore{secret: []byte("private-key")}
	deepSeek := &profileDeepSeekClient{}
	kimi := &profileDeepSeekClient{reply: "Kimi reply"}
	miniMax := &profileDeepSeekClient{reply: "MiniMax reply"}
	records := map[string]projection.ProviderCredentialRecord{
		"kimi": {
			ProviderID: "kimi", ProviderAccountID: "kimi.primary",
			CredentialReference: "credential-ref-kimi-1",
			Revision:            4, Status: "verified", AuthMode: "brokered",
		},
		"minimax": {
			ProviderID: "minimax", ProviderAccountID: "minimax.primary",
			CredentialReference: "credential-ref-minimax-1",
			Revision:            6, Status: "verified", AuthMode: "brokered",
		},
	}
	router, err := newProductConversationProfileRouter(
		nil,
		func(providerID string) []projection.ProviderCredentialRecord {
			record, ok := records[providerID]
			if !ok {
				return nil
			}
			return []projection.ProviderCredentialRecord{record}
		},
		profileConversationLeasing(t, store),
		deepSeek,
		productConversationProviderRoute{
			providerID: "kimi",
			profileID:  provider.KimiConversationAccountProfileID,
			client:     kimi,
		},
		productConversationProviderRoute{
			providerID: "minimax",
			profileID:  provider.MiniMaxConversationAccountProfileID,
			client:     miniMax,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		profileID string
		response  string
		client    *profileDeepSeekClient
	}{
		{provider.KimiConversationProfileID(4), "Kimi reply", kimi},
		{provider.MiniMaxConversationProfileID(6), "MiniMax reply", miniMax},
	} {
		response, respondErr := router.Respond(
			context.Background(),
			api.LocalProductConversationRequest{
				ThreadID: "thread-provider", ProfileID: test.profileID,
				Messages: []api.LocalProductChatMessage{{Role: "user", Content: "hello"}},
			},
		)
		if respondErr != nil || response.Content != test.response ||
			!response.Tentative || len(test.client.messages) != 1 {
			t.Fatalf("profile=%s response=%#v error=%v client=%#v", test.profileID, response, respondErr, test.client)
		}
	}
	if len(deepSeek.messages) != 0 || store.reads != 2 {
		t.Fatalf("DeepSeek=%#v reads=%d", deepSeek, store.reads)
	}
	if _, err := router.Respond(
		context.Background(),
		api.LocalProductConversationRequest{
			ThreadID: "thread-provider", ProfileID: provider.KimiConversationProfileID(5),
			Messages: []api.LocalProductChatMessage{{Role: "user", Content: "stale"}},
		},
	); err == nil || store.reads != 2 {
		t.Fatalf("stale profile error=%v reads=%d", err, store.reads)
	}
}

func TestConversationProfileRouterRequiresExplicitCodexProfile(t *testing.T) {
	calls := 0
	defaultResponder := profileConversationResponderFunc(func(
		context.Context,
		api.LocalProductConversationRequest,
	) (api.LocalProductConversationResponse, error) {
		calls++
		return api.LocalProductConversationResponse{Content: "Codex reply", Tentative: true}, nil
	})
	router, err := newProductConversationProfileRouter(
		defaultResponder,
		func(string) []projection.ProviderCredentialRecord {
			return nil
		},
		profileConversationLeasing(t, &profileConversationStore{secret: []byte("unused")}),
		&profileDeepSeekClient{},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := router.Respond(context.Background(), api.LocalProductConversationRequest{
		ThreadID: "thread-codex", ProfileID: "",
		Messages: []api.LocalProductChatMessage{{Role: "user", Content: "hello"}},
	}); !errors.Is(err, api.ErrLocalProductChatUnavailable) {
		t.Fatalf("blank profile error = %v", err)
	}
	if _, err := router.ResolveConversationExecutionBinding(
		context.Background(), "", "",
	); !errors.Is(err, api.ErrLocalProductChatUnavailable) {
		t.Fatalf("blank binding error = %v", err)
	}
	response, err := router.Respond(context.Background(), api.LocalProductConversationRequest{
		ThreadID: "thread-codex", ProfileID: provider.CodexConversationProfileID,
		Messages: []api.LocalProductChatMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil || response.Content != "Codex reply" {
		t.Fatalf("explicit Codex response=%#v error=%v", response, err)
	}
	if calls != 1 {
		t.Fatalf("default calls = %d", calls)
	}
}

func TestConversationProfileRouterRequiresExplicitPiProfile(t *testing.T) {
	router := &productConversationProfileRouter{
		defaultResponder: &productPiConversationResponder{},
	}
	binding, err := router.ResolveConversationExecutionBinding(
		context.Background(), provider.PiConversationProfileID, "",
	)
	if err != nil || binding.ProviderID != "loom-local" ||
		binding.SchemaVersion != 4 || binding.HarnessAdapter != "pi" {
		t.Fatalf("Pi binding = %#v, %v", binding, err)
	}
	if _, err := router.ResolveConversationExecutionBinding(
		context.Background(), "", "",
	); !errors.Is(err, api.ErrLocalProductChatUnavailable) {
		t.Fatalf("blank Pi binding error = %v", err)
	}
}

var _ credentials.SecretStore = (*profileConversationStore)(nil)

func profileConversationLeasing(
	t testing.TB,
	store credentials.SecretStore,
) productCredentialLeaseAccess {
	t.Helper()
	leasing, err := newProductLegacyCredentialLeaseAccess(store)
	if err != nil {
		t.Fatal(err)
	}
	return leasing
}

func TestConversationProfileRouterRoutesOpenCodeProfileToDedicatedResponder(t *testing.T) {
	defaultCalls := 0
	openCodeCalls := 0
	var gotModelID string
	defaultResponder := profileConversationResponderFunc(func(
		context.Context,
		api.LocalProductConversationRequest,
	) (api.LocalProductConversationResponse, error) {
		defaultCalls++
		return api.LocalProductConversationResponse{}, errors.New("default responder must not serve OpenCode")
	})
	openCodeResponder := profileConversationResponderFunc(func(
		_ context.Context,
		request api.LocalProductConversationRequest,
	) (api.LocalProductConversationResponse, error) {
		openCodeCalls++
		gotModelID = request.ModelID
		return api.LocalProductConversationResponse{Content: "OpenCode reply", Tentative: true}, nil
	})
	router, err := newProductConversationProfileRouterWithPolicy(
		defaultResponder,
		func(string) []projection.ProviderCredentialRecord {
			return nil
		},
		func(string, string) (work.ProviderAccountPolicy, bool) {
			return work.ProviderAccountPolicy{}, false
		},
		profileConversationLeasing(t, &profileConversationStore{secret: []byte("unused")}),
		&profileDeepSeekClient{}, false, openCodeResponder, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	response, err := router.Respond(context.Background(), api.LocalProductConversationRequest{
		ThreadID:  "thread-opencode",
		ProfileID: provider.OpenCodeConversationProfileID,
		ModelID:   provider.OpenCodeConversationDefaultModel,
		Messages:  []api.LocalProductChatMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil || response.Content != "OpenCode reply" || !response.Tentative ||
		defaultCalls != 0 || openCodeCalls != 1 ||
		gotModelID != provider.OpenCodeConversationDefaultModel {
		t.Fatalf(
			"response=%#v error=%v default_calls=%d opencode_calls=%d model=%q",
			response, err, defaultCalls, openCodeCalls, gotModelID,
		)
	}
}

func TestConversationProfileRouterResolvesOpenCodeBinding(t *testing.T) {
	router, err := newProductConversationProfileRouter(
		profileConversationResponderFunc(func(
			context.Context,
			api.LocalProductConversationRequest,
		) (api.LocalProductConversationResponse, error) {
			return api.LocalProductConversationResponse{}, errors.New("unexpected")
		}),
		func(string) []projection.ProviderCredentialRecord {
			return nil
		},
		profileConversationLeasing(t, &profileConversationStore{secret: []byte("unused")}),
		&profileDeepSeekClient{},
	)
	if err != nil {
		t.Fatal(err)
	}
	binding, err := router.ResolveConversationExecutionBinding(
		context.Background(), provider.OpenCodeConversationProfileID,
		provider.OpenCodeConversationDefaultModel,
	)
	if err != nil || binding.SchemaVersion != 4 || binding.ProviderID != "opencode" ||
		binding.HarnessAdapter != "opencode" ||
		binding.ModelID != provider.OpenCodeConversationDefaultModel {
		t.Fatalf("binding=%#v error=%v", binding, err)
	}
	target, err := router.ResolveConversationContextTarget(
		context.Background(), "conversation-opencode", "segment-1",
		provider.OpenCodeConversationProfileID,
		provider.OpenCodeConversationDefaultModel,
	)
	if err != nil || target.ConversationID != "conversation-opencode" ||
		target.ProviderID != "opencode" || target.AuthMode != "native_auth" ||
		target.ContextAdapterID != "context:loom-native:v1" ||
		target.DisclosurePolicyVersion != 1 {
		t.Fatalf("target=%#v error=%v", target, err)
	}
}

func TestConversationProfileRouterRoutesNativeClaudeCodeWithoutCredentialLease(
	t *testing.T,
) {
	lease := &profileConversationLeaseRecorder{}
	claudeCalls := 0
	var got api.LocalProductConversationRequest
	router, err := newProductConversationProfileRouterWithPolicy(
		profileConversationResponderFunc(func(
			context.Context,
			api.LocalProductConversationRequest,
		) (api.LocalProductConversationResponse, error) {
			return api.LocalProductConversationResponse{}, errors.New("unexpected default responder")
		}),
		func(string) []projection.ProviderCredentialRecord { return nil },
		func(string, string) (work.ProviderAccountPolicy, bool) {
			return work.ProviderAccountPolicy{}, false
		},
		lease, &profileDeepSeekClient{}, true, nil, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	router.claudeCodeResponder = profileConversationResponderFunc(func(
		_ context.Context,
		request api.LocalProductConversationRequest,
	) (api.LocalProductConversationResponse, error) {
		claudeCalls++
		got = request
		return api.LocalProductConversationResponse{
			Content: "Claude Code reply", Tentative: true,
		}, nil
	})
	binding, err := router.ResolveConversationExecutionBinding(
		context.Background(), provider.ClaudeCodeConversationProfileID,
		provider.AnthropicConversationModelID,
	)
	if err != nil || binding.SchemaVersion != 4 ||
		binding.HarnessAdapter != "claude-code" || binding.ProviderID != "anthropic" ||
		binding.ProviderAccountID != "" || binding.CredentialRevision != 0 ||
		binding.ModelID != provider.AnthropicConversationModelID {
		t.Fatalf("Claude Code binding=%#v error=%v", binding, err)
	}
	target, err := router.ResolveConversationContextTarget(
		context.Background(), "conversation-claude", "segment-1",
		provider.ClaudeCodeConversationProfileID,
		provider.AnthropicConversationModelID,
	)
	if err != nil || target.ProviderID != "anthropic" ||
		target.ProviderAccountID != "" || target.AuthMode != "native_auth" ||
		target.ContextAdapterID != "context:claude-code:v1" {
		t.Fatalf("Claude Code target=%#v error=%v", target, err)
	}
	response, err := router.Respond(
		context.Background(), api.LocalProductConversationRequest{
			ThreadID: "thread-claude", ProfileID: provider.ClaudeCodeConversationProfileID,
			ModelID:          provider.AnthropicConversationModelID,
			ExecutionBinding: &binding,
			Messages:         []api.LocalProductChatMessage{{Role: "user", Content: "hello"}},
		},
	)
	if err != nil || response.Content != "Claude Code reply" || !response.Tentative ||
		claudeCalls != 1 || got.ExecutionBinding == nil ||
		*got.ExecutionBinding != binding || lease.calls != 0 {
		t.Fatalf(
			"response=%#v error=%v calls=%d request=%#v lease_calls=%d",
			response, err, claudeCalls, got, lease.calls,
		)
	}
}

func TestConversationProfileRouterPreservesSpecificResponderFailure(t *testing.T) {
	openCodeResponder := profileConversationResponderFunc(func(
		context.Context,
		api.LocalProductConversationRequest,
	) (api.LocalProductConversationResponse, error) {
		return api.LocalProductConversationResponse{},
			api.NewLocalProductConversationDispatchErrorWithDetails(
				api.LocalProductConversationDispatchFailureInfo{
					Code: "provider_auth", Stage: "provider_auth",
					UserMessage: "The selected model requires a verified openai Provider credential.",
					Retryable:   false,
				},
				errors.New("no credential"),
			)
	})
	router, err := newProductConversationProfileRouterWithPolicy(
		profileConversationResponderFunc(func(
			context.Context,
			api.LocalProductConversationRequest,
		) (api.LocalProductConversationResponse, error) {
			return api.LocalProductConversationResponse{}, errors.New("unexpected default responder")
		}),
		func(string) []projection.ProviderCredentialRecord {
			return nil
		},
		func(string, string) (work.ProviderAccountPolicy, bool) {
			return work.ProviderAccountPolicy{}, false
		},
		profileConversationLeasing(t, &profileConversationStore{secret: []byte("unused")}),
		&profileDeepSeekClient{}, false, openCodeResponder, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	_, err = router.Respond(context.Background(), api.LocalProductConversationRequest{
		ThreadID:  "thread-opencode-preserve",
		ProfileID: provider.OpenCodeConversationProfileID,
		ModelID:   provider.OpenCodeConversationDefaultModel,
		Messages:  []api.LocalProductChatMessage{{Role: "user", Content: "hello"}},
	})
	code, stage, retryable, ok := api.LocalProductConversationDispatchFailure(err)
	if !ok || code != "provider_auth" || stage != "provider_auth" || retryable {
		t.Fatalf("error code=%s stage=%s retryable=%v ok=%v", code, stage, retryable, ok)
	}
}

func TestConversationProfileRouterFreezesExactOpenCodeProviderAccount(t *testing.T) {
	client := &openCodeResponderClientFixture{content: "OPENAI-ACCOUNT-OK"}
	lease := &openCodeResponderLeaseFixture{}
	records := []projection.ProviderCredentialRecord{
		{
			ProviderID: "openai", ProviderAccountID: "openai.primary",
			CredentialReference: "credential-ref-openai-primary", Revision: 7,
			Status: string(credentials.CredentialVerified), AuthMode: "brokered",
		},
		{
			ProviderID: "openai", ProviderAccountID: "openai.team",
			CredentialReference: "credential-ref-openai-team", Revision: 9,
			Status: string(credentials.CredentialVerified), AuthMode: "brokered",
		},
	}
	responder := &productOpenCodeConversationResponder{client: client, leases: lease}
	defaultCalls := 0
	router, err := newProductConversationProfileRouterWithPolicy(
		profileConversationResponderFunc(func(
			context.Context,
			api.LocalProductConversationRequest,
		) (api.LocalProductConversationResponse, error) {
			defaultCalls++
			return api.LocalProductConversationResponse{}, errors.New("default responder must not serve brokered OpenCode")
		}),
		func(providerID string) []projection.ProviderCredentialRecord {
			if providerID != "openai" {
				return nil
			}
			return records
		},
		func(string, string) (work.ProviderAccountPolicy, bool) {
			return work.ProviderAccountPolicy{}, false
		},
		lease, &profileDeepSeekClient{}, true, responder, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	profileID := provider.OpenCodeConversationAccountProfileID(
		"openai", "openai.team", 9,
	)
	binding, err := router.ResolveConversationExecutionBinding(
		context.Background(), profileID, "openai/gpt-5.5",
	)
	if err != nil {
		t.Fatal(err)
	}
	response, err := router.Respond(
		context.Background(),
		api.LocalProductConversationRequest{
			ThreadID: "thread-opencode-openai-team", ProfileID: profileID,
			ModelID: "openai/gpt-5.5", ExecutionBinding: &binding,
			Messages: []api.LocalProductChatMessage{{Role: "user", Content: "hello"}},
		},
	)
	if err != nil || response.Content != "OPENAI-ACCOUNT-OK" || defaultCalls != 0 {
		t.Fatalf("response=%#v error=%v default_calls=%d", response, err, defaultCalls)
	}
	if lease.identity.ProviderID != "openai" ||
		lease.identity.ProviderAccountID != "openai.team" ||
		lease.identity.CredentialReference != "credential-ref-openai-team" ||
		lease.identity.CredentialRevision != 9 ||
		client.envName != "OPENAI_API_KEY" {
		t.Fatalf("identity=%#v env=%q", lease.identity, client.envName)
	}
	if binding.HarnessAdapter != "opencode" || binding.ProviderID != "openai" ||
		binding.ProviderAccountID != "openai.team" ||
		binding.CredentialRevision != 9 || binding.ModelID != "openai/gpt-5.5" {
		t.Fatalf("binding=%#v", binding)
	}
}

func TestConversationProfileRouterRoutesCodexToCodexResponder(t *testing.T) {
	opencodeCalls := 0
	codexCalls := 0
	opencodeResponder := profileConversationResponderFunc(func(
		context.Context,
		api.LocalProductConversationRequest,
	) (api.LocalProductConversationResponse, error) {
		opencodeCalls++
		return api.LocalProductConversationResponse{}, errors.New("opencode must not serve codex")
	})
	codexResponder := profileConversationResponderFunc(func(
		_ context.Context,
		request api.LocalProductConversationRequest,
	) (api.LocalProductConversationResponse, error) {
		codexCalls++
		if request.ModelID != "codex-default" {
			return api.LocalProductConversationResponse{}, errors.New("wrong model")
		}
		return api.LocalProductConversationResponse{Content: "Codex reply", Tentative: true}, nil
	})
	router, err := newProductConversationProfileRouterWithPolicy(
		nil,
		func(string) []projection.ProviderCredentialRecord {
			return nil
		},
		func(string, string) (work.ProviderAccountPolicy, bool) {
			return work.ProviderAccountPolicy{}, false
		},
		profileConversationLeasing(t, &profileConversationStore{secret: []byte("unused")}),
		&profileDeepSeekClient{}, true, opencodeResponder, codexResponder,
	)
	if err != nil {
		t.Fatal(err)
	}
	binding := api.LocalProductConversationExecutionBinding{
		SchemaVersion: 4, HarnessAdapter: "codex", ProviderID: "openai",
		ModelID: "codex-default",
	}
	response, err := router.Respond(context.Background(), api.LocalProductConversationRequest{
		ThreadID:         "thread-codex-router",
		ProfileID:        provider.CodexConversationProfileID,
		ModelID:          "codex-default",
		ExecutionBinding: &binding,
		Messages:         []api.LocalProductChatMessage{{Role: "user", Content: "hello"}},
	})
	if err != nil || response.Content != "Codex reply" || codexCalls != 1 ||
		opencodeCalls != 0 {
		t.Fatalf("response=%#v err=%v codexCalls=%d opencodeCalls=%d",
			response, err, codexCalls, opencodeCalls)
	}
}

func TestConversationProfileRouterFreezesExplicitCodexModels(t *testing.T) {
	router := &productConversationProfileRouter{}
	for _, modelID := range []string{"gpt-5.6-sol", "gpt-5.5", "gpt-5.4-mini"} {
		t.Run(modelID, func(t *testing.T) {
			binding, err := router.ResolveConversationExecutionBinding(
				context.Background(), provider.CodexConversationProfileID, modelID,
			)
			if err != nil || binding.HarnessAdapter != "codex" ||
				binding.ProviderID != "openai" || binding.ModelID != modelID {
				t.Fatalf("binding=%#v error=%v", binding, err)
			}
			target, err := router.ResolveConversationContextTarget(
				context.Background(), "thread-codex-model", "segment-1",
				provider.CodexConversationProfileID, modelID,
			)
			if err != nil || target.ProviderID != "openai" || target.ModelID != modelID {
				t.Fatalf("target=%#v error=%v", target, err)
			}
		})
	}
	for _, invalid := range []string{"deepseek-v4-flash", "deepseek/deepseek-v4-flash", "unknown-model"} {
		if _, err := router.ResolveConversationExecutionBinding(
			context.Background(), provider.CodexConversationProfileID, invalid,
		); !errors.Is(err, api.ErrLocalProductChatUnavailable) {
			t.Fatalf("invalid model %q error = %v", invalid, err)
		}
	}
}

func TestProductConversationCapacityUsesExactProviderAndModelCatalogKeys(t *testing.T) {
	router := &productConversationProfileRouter{}
	for _, test := range []struct {
		name       string
		providerID string
		modelID    string
		status     contextcapsule.CapacityStatus
		window     int
		reserved   int
		overhead   int
	}{
		{name: "deepseek v4 flash", providerID: "deepseek", modelID: "deepseek-v4-flash", status: contextcapsule.CapacityEstimated, window: 1_000_000, reserved: 384_000, overhead: 1_024},
		{name: "deepseek v4 pro", providerID: "deepseek", modelID: "deepseek-v4-pro", status: contextcapsule.CapacityEstimated, window: 1_000_000, reserved: 384_000, overhead: 1_024},
		{name: "anthropic sonnet 5", providerID: "anthropic", modelID: "claude-sonnet-5", status: contextcapsule.CapacityEstimated, window: 1_000_000, reserved: 128_000, overhead: 1_024},
		{name: "openai sol", providerID: "openai", modelID: "gpt-5.6-sol", status: contextcapsule.CapacityEstimated, window: 1_050_000, reserved: 128_000, overhead: 1_024},
		{name: "openai mini", providerID: "openai", modelID: "gpt-5.4-mini", status: contextcapsule.CapacityEstimated, window: 400_000, reserved: 128_000, overhead: 1_024},
		{name: "uncertain deepseek alias", providerID: "deepseek", modelID: "deepseek-chat", status: contextcapsule.CapacityUnavailable},
		{name: "missing minimax output ceiling", providerID: "minimax", modelID: "MiniMax-M3", status: contextcapsule.CapacityUnavailable},
		{name: "missing kimi output ceiling", providerID: "kimi", modelID: "kimi-k2.6", status: contextcapsule.CapacityUnavailable},
		{name: "provider model substitution", providerID: "deepseek", modelID: "gpt-5.6-sol", status: contextcapsule.CapacityUnavailable},
		{name: "harness model alias is not provider capacity", providerID: "opencode", modelID: "deepseek/deepseek-v4-flash", status: contextcapsule.CapacityUnavailable},
		{name: "unknown model", providerID: "anthropic", modelID: "claude-future", status: contextcapsule.CapacityUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			target := contextcapsule.Target{
				ProviderID: test.providerID, ModelID: test.modelID,
				AuthMode: "native_auth", TokenBudget: 8_192,
			}
			authority, counter, err := router.ResolveConversationContextCapacity(
				context.Background(), target,
			)
			if err != nil {
				t.Fatal(err)
			}
			if authority.Status != test.status ||
				authority.ContextWindowTokens != test.window ||
				authority.ReservedOutputTokens != test.reserved ||
				authority.AdapterToolOverheadTokens != test.overhead ||
				counter == nil || authority.TokenCounterID != counter.ID() ||
				authority.TokenCounterVersion != counter.Version() {
				t.Fatalf("authority=%#v counter=%#v", authority, counter)
			}
		})
	}
}

func TestProductConversationCapacityCounterIsExplicitlyHeuristic(t *testing.T) {
	router := &productConversationProfileRouter{}
	authority, counter, err := router.ResolveConversationContextCapacity(
		context.Background(),
		contextcapsule.Target{
			ProviderID: "deepseek", ModelID: "deepseek-v4-flash",
			AuthMode: "native_auth", TokenBudget: 8_192,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if authority.Status != contextcapsule.CapacityEstimated ||
		counter.ID() != "loom-unicode-rune-quarter-estimate" ||
		counter.Version() != "v1" ||
		authority.TokenCounterID != counter.ID() ||
		authority.TokenCounterVersion != counter.Version() {
		t.Fatalf("authority=%#v counter=%#v", authority, counter)
	}
	for _, test := range []struct {
		content string
		want    int
	}{
		{content: "", want: 1},
		{content: "abcd", want: 1},
		{content: "abcde", want: 2},
		{content: "你好世界", want: 1},
	} {
		got, countErr := counter.CountTokens([]byte(test.content))
		if countErr != nil || got != test.want {
			t.Fatalf("CountTokens(%q)=%d, %v want %d", test.content, got, countErr, test.want)
		}
	}
}
