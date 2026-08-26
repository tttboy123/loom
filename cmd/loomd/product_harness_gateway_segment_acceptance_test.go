package main

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/harnessgateway"
	"loom-pi-rebuild/internal/runtime/harnessadapter"
)

const (
	productGatewaySegmentAcceptanceThreadID      = "adr-0022-visible-conversation"
	productGatewaySegmentAcceptanceCodexProfile  = "conversation-openai-codex"
	productGatewaySegmentAcceptanceCodexModel    = "gpt-5.6-codex"
	productGatewaySegmentAcceptanceNativeProfile = "conversation-deepseek-loom-native"
	productGatewaySegmentAcceptanceNativeModel   = "deepseek-chat"
)

func TestADR0022FirstLoopReusesCodexSegmentThenSwitchesToLoomNativeThroughGateway(
	t *testing.T,
) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	privateRoot := filepath.Join(root, "codex-private")
	for _, path := range []string{workspace, privateRoot} {
		if err := prepareProductHarnessGatewayWorkspace(path); err != nil {
			t.Fatal(err)
		}
	}

	nativeExecutor := &productGatewaySegmentAcceptanceExecutor{}
	events := &productGatewaySegmentAcceptanceEventSink{}
	codexRuntime := &productGatewaySegmentAcceptanceCodexRuntime{}
	var openedMu sync.Mutex
	var opened []harnessadapter.CodexSegmentSessionConfig
	responder, err := newProductHarnessGatewayConversationResponder(
		productHarnessGatewayConversationConfig{
			Executor: nativeExecutor, WorkspacePath: workspace, Events: events,
			Now: productGatewaySegmentAcceptanceClock,
			CodexSegment: &productCodexSegmentBackendConfig{
				ExecutablePath: "/opt/loom/bin/codex",
				HomePath:       filepath.Join(root, "home"),
				PrivateRoot:    privateRoot,
				Sessions:       productGatewaySegmentAcceptanceSessionRunner{},
				Open: func(
					_ context.Context,
					config harnessadapter.CodexSegmentSessionConfig,
				) (productCodexSegmentRuntime, error) {
					openedMu.Lock()
					opened = append(opened, config)
					openedMu.Unlock()
					return codexRuntime, nil
				},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := responder.Close(context.Background()); closeErr != nil {
			t.Errorf("close Harness Gateway responder: %v", closeErr)
		}
	})

	chat, err := api.NewPersistentLocalProductChatAPI(
		filepath.Join(root, "chat.json"), productGatewaySegmentAcceptanceClock, responder,
	)
	if err != nil {
		t.Fatal(err)
	}
	codexBinding := productGatewaySegmentAcceptanceCodexBinding()
	nativeBinding := productGatewaySegmentAcceptanceNativeBinding()
	resolver := &productGatewaySegmentAcceptanceBindingResolver{
		bindings: map[string]api.LocalProductConversationExecutionBinding{
			productGatewaySegmentAcceptanceCodexProfile:  codexBinding,
			productGatewaySegmentAcceptanceNativeProfile: nativeBinding,
		},
	}
	if err := chat.SetConversationExecutionBindingResolver(resolver); err != nil {
		t.Fatal(err)
	}
	if err := chat.SetConversationContextCapsuleRuntime(
		resolver, &productGatewaySegmentAcceptanceCapsuleStore{},
	); err != nil {
		t.Fatal(err)
	}

	first := productGatewaySegmentAcceptanceSend(t, chat, api.LocalProductChatMessageRequest{
		ThreadID:        productGatewaySegmentAcceptanceThreadID,
		Content:         "codex source turn alpha",
		ProfileID:       productGatewaySegmentAcceptanceCodexProfile,
		ModelID:         productGatewaySegmentAcceptanceCodexModel,
		ReasoningEffort: "high",
		IncidentID:      "incident-codex-alpha",
	})
	if len(first.Segments) != 1 || len(first.Attempts) != 1 || len(first.Messages) != 2 {
		t.Fatalf("first Codex turn thread = %#v", first)
	}
	frozenAfterFirst := first.Segments[0]

	second := productGatewaySegmentAcceptanceSend(t, chat, api.LocalProductChatMessageRequest{
		ThreadID:        productGatewaySegmentAcceptanceThreadID,
		Content:         "codex source turn beta",
		ProfileID:       productGatewaySegmentAcceptanceCodexProfile,
		ModelID:         productGatewaySegmentAcceptanceCodexModel,
		ReasoningEffort: "high",
		IncidentID:      "incident-codex-beta",
	})
	if len(second.Segments) != 1 || len(second.Attempts) != 2 || len(second.Messages) != 4 {
		t.Fatalf("second Codex turn thread = %#v", second)
	}
	if !reflect.DeepEqual(second.Segments[0], frozenAfterFirst) {
		t.Fatalf("Codex Segment changed between turns:\n first=%#v\n second=%#v", frozenAfterFirst, second.Segments[0])
	}
	frozenCodexSegment := second.Segments[0]

	openedMu.Lock()
	openedSnapshot := append([]harnessadapter.CodexSegmentSessionConfig(nil), opened...)
	openedMu.Unlock()
	if len(openedSnapshot) != 1 {
		t.Fatalf("Codex Segment runtime opens = %d, want 1", len(openedSnapshot))
	}
	if openedSnapshot[0].WorkspacePath != workspace ||
		openedSnapshot[0].ModelID != productGatewaySegmentAcceptanceCodexModel ||
		openedSnapshot[0].ReasoningEffort != "high" {
		t.Fatalf("Codex Segment runtime config = %#v", openedSnapshot[0])
	}
	prompts := codexRuntime.promptsSnapshot()
	if len(prompts) != 2 ||
		!strings.Contains(prompts[0], "codex source turn alpha") ||
		prompts[1] != "codex source turn beta" {
		t.Fatalf("Codex Segment prompts = %#v", prompts)
	}
	if got := len(nativeExecutor.requestsSnapshot()); got != 0 {
		t.Fatalf("native executor calls during Codex Segment = %d, want 0", got)
	}

	acknowledgement, err := api.NewLocalProductConversationTrustBoundaryAcknowledgement(
		second.ThreadID, frozenCodexSegment, productGatewaySegmentAcceptanceNativeProfile,
		nativeBinding, "", api.ContextModeSummaryOnly,
	)
	if err != nil {
		t.Fatal(err)
	}
	transitioned := productGatewaySegmentAcceptanceSend(t, chat, api.LocalProductChatMessageRequest{
		ThreadID:                     productGatewaySegmentAcceptanceThreadID,
		Content:                      "deepseek target turn gamma",
		ProfileID:                    productGatewaySegmentAcceptanceNativeProfile,
		ModelID:                      productGatewaySegmentAcceptanceNativeModel,
		ContextMode:                  api.ContextModeSummaryOnly,
		ExpectedExecutionBinding:     &nativeBinding,
		TrustBoundaryAcknowledgement: &acknowledgement,
		IncidentID:                   "incident-native-gamma",
	})
	if transitioned.ThreadID != productGatewaySegmentAcceptanceThreadID ||
		transitioned.ProfileID != productGatewaySegmentAcceptanceNativeProfile ||
		len(transitioned.Segments) != 2 || len(transitioned.Attempts) != 3 ||
		len(transitioned.Messages) != 6 {
		t.Fatalf("transitioned thread = %#v", transitioned)
	}
	if !reflect.DeepEqual(transitioned.Segments[0], frozenCodexSegment) {
		t.Fatalf("source Segment changed after route transition:\n before=%#v\n after=%#v", frozenCodexSegment, transitioned.Segments[0])
	}
	targetSegment := transitioned.Segments[1]
	if targetSegment.SegmentID == frozenCodexSegment.SegmentID ||
		targetSegment.ProfileID != productGatewaySegmentAcceptanceNativeProfile ||
		targetSegment.ModelID != productGatewaySegmentAcceptanceNativeModel ||
		targetSegment.ContextMode != api.ContextModeSummaryOnly ||
		targetSegment.RouteTransitionReviewDigest != acknowledgement.ReviewDigest ||
		targetSegment.ExecutionBinding == nil || *targetSegment.ExecutionBinding != nativeBinding {
		t.Fatalf("target Segment = %#v", targetSegment)
	}
	targetAttempt := transitioned.Attempts[2]
	if targetAttempt.SegmentID != targetSegment.SegmentID ||
		targetAttempt.ProfileID != targetSegment.ProfileID ||
		targetAttempt.ModelID != targetSegment.ModelID ||
		targetAttempt.ContextMode != targetSegment.ContextMode ||
		targetAttempt.ContextCapsuleDigest != targetSegment.ContextCapsuleDigest ||
		targetAttempt.BindingDigest != targetSegment.BindingDigest ||
		targetAttempt.RouteTransitionReviewDigest != targetSegment.RouteTransitionReviewDigest ||
		targetAttempt.ExecutionBinding == nil || *targetAttempt.ExecutionBinding != nativeBinding ||
		targetAttempt.Status != "succeeded" {
		t.Fatalf("target attempt = %#v", targetAttempt)
	}
	nativeRequests := nativeExecutor.requestsSnapshot()
	if len(nativeRequests) != 1 || nativeRequests[0].ThreadID != transitioned.ThreadID ||
		nativeRequests[0].SegmentID != targetSegment.SegmentID ||
		nativeRequests[0].ContextMode != api.ContextModeSummaryOnly ||
		nativeRequests[0].ContextCapsuleDigest != targetSegment.ContextCapsuleDigest ||
		nativeRequests[0].SegmentContextCapsuleDigest != targetSegment.ContextCapsuleDigest ||
		nativeRequests[0].RouteTransitionReviewDigest != acknowledgement.ReviewDigest ||
		nativeRequests[0].ExecutionBinding == nil || *nativeRequests[0].ExecutionBinding != nativeBinding {
		t.Fatalf("Loom Native request = %#v", nativeRequests)
	}

	continued := productGatewaySegmentAcceptanceSend(t, chat, api.LocalProductChatMessageRequest{
		ThreadID:   productGatewaySegmentAcceptanceThreadID,
		Content:    "deepseek target turn delta",
		ProfileID:  productGatewaySegmentAcceptanceNativeProfile,
		ModelID:    productGatewaySegmentAcceptanceNativeModel,
		IncidentID: "incident-native-delta",
	})
	if continued.ThreadID != transitioned.ThreadID || len(continued.Segments) != 2 ||
		len(continued.Attempts) != 4 || len(continued.Messages) != 8 ||
		!reflect.DeepEqual(continued.Segments[0], frozenCodexSegment) ||
		!reflect.DeepEqual(continued.Segments[1], targetSegment) {
		t.Fatalf("continued target thread = %#v", continued)
	}
	continuedAttempt := continued.Attempts[3]
	if continuedAttempt.SegmentID != targetSegment.SegmentID ||
		continuedAttempt.ContextCapsuleDigest == targetSegment.ContextCapsuleDigest ||
		continuedAttempt.BindingDigest == targetSegment.BindingDigest ||
		continuedAttempt.ExecutionBinding == nil || *continuedAttempt.ExecutionBinding != nativeBinding ||
		continuedAttempt.Status != "succeeded" {
		t.Fatalf("continued target attempt = %#v, target Segment = %#v", continuedAttempt, targetSegment)
	}
	nativeRequests = nativeExecutor.requestsSnapshot()
	if len(nativeRequests) != 2 ||
		nativeRequests[1].SegmentID != targetSegment.SegmentID ||
		nativeRequests[1].ContextCapsuleDigest != continuedAttempt.ContextCapsuleDigest ||
		nativeRequests[1].SegmentContextCapsuleDigest != targetSegment.ContextCapsuleDigest ||
		nativeRequests[1].RouteTransitionReviewDigest != acknowledgement.ReviewDigest {
		t.Fatalf("continued Loom Native requests = %#v", nativeRequests)
	}

	productGatewaySegmentAcceptanceAssertMessages(t, continued, targetSegment.SegmentID)
	if err := responder.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	productGatewaySegmentAcceptanceAssertSessionsAndEvents(t, events.snapshot())
}

func TestADR0022GatewayAllowsDifferentConversationsToOverlap(t *testing.T) {
	root := t.TempDir()
	executor := newProductGatewaySegmentAcceptanceOverlapExecutor()
	events := &productGatewaySegmentAcceptanceEventSink{}
	responder, err := newProductHarnessGatewayConversationResponder(
		productHarnessGatewayConversationConfig{
			Executor: executor, WorkspacePath: root, Events: events,
			Now: productGatewaySegmentAcceptanceClock,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := responder.Close(context.Background()); closeErr != nil {
			t.Errorf("close Harness Gateway responder: %v", closeErr)
		}
	})
	chat, err := api.NewPersistentLocalProductChatAPI(
		filepath.Join(root, "overlap-chat.json"), productGatewaySegmentAcceptanceClock, responder,
	)
	if err != nil {
		t.Fatal(err)
	}
	binding := productGatewaySegmentAcceptanceNativeBinding()
	resolver := &productGatewaySegmentAcceptanceBindingResolver{
		bindings: map[string]api.LocalProductConversationExecutionBinding{
			productGatewaySegmentAcceptanceNativeProfile: binding,
		},
	}
	if err := chat.SetConversationExecutionBindingResolver(resolver); err != nil {
		t.Fatal(err)
	}

	type result struct {
		thread api.LocalProductChatThread
		err    error
	}
	results := make(chan result, 2)
	for index := 1; index <= 2; index++ {
		index := index
		go func() {
			thread, sendErr := chat.SendMessage(context.Background(), api.LocalProductChatMessageRequest{
				ThreadID:   fmt.Sprintf("overlap-conversation-%d", index),
				Content:    fmt.Sprintf("overlap private turn %d", index),
				ProfileID:  productGatewaySegmentAcceptanceNativeProfile,
				ModelID:    productGatewaySegmentAcceptanceNativeModel,
				IncidentID: fmt.Sprintf("incident-overlap-%d", index),
			})
			results <- result{thread: thread, err: sendErr}
		}()
	}

	select {
	case <-executor.bothStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("different Conversations did not overlap in the Gateway executor")
	}
	close(executor.release)
	for range 2 {
		select {
		case got := <-results:
			if got.err != nil {
				t.Fatal(got.err)
			}
			if len(got.thread.Segments) != 1 || len(got.thread.Attempts) != 1 ||
				len(got.thread.Messages) != 2 || got.thread.Attempts[0].Status != "succeeded" {
				t.Fatalf("overlapping Conversation result = %#v", got.thread)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("overlapping Conversation did not complete")
		}
	}
	if err := responder.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	eventSnapshot := events.snapshot()
	openingSessions := make(map[string]string)
	for _, event := range eventSnapshot {
		if event.Type == harnessgateway.EventSessionOpening {
			openingSessions[event.ConversationID] = event.SessionID
		}
	}
	if len(openingSessions) != 2 ||
		openingSessions["overlap-conversation-1"] == openingSessions["overlap-conversation-2"] {
		t.Fatalf("overlapping Gateway sessions = %#v", openingSessions)
	}
	productGatewaySegmentAcceptanceAssertContentFreeEvents(t, eventSnapshot)
}

func productGatewaySegmentAcceptanceSend(
	t *testing.T,
	chat *api.LocalProductChatAPI,
	request api.LocalProductChatMessageRequest,
) api.LocalProductChatThread {
	t.Helper()
	thread, err := chat.SendMessage(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	return thread
}

func productGatewaySegmentAcceptanceAssertMessages(
	t *testing.T,
	thread api.LocalProductChatThread,
	targetSegmentID string,
) {
	t.Helper()
	want := []struct {
		role      string
		content   string
		segmentID string
		tentative bool
	}{
		{"user", "codex source turn alpha", "segment-1", false},
		{"loom", "codex-runtime-reply-1", "segment-1", true},
		{"user", "codex source turn beta", "segment-1", false},
		{"loom", "codex-runtime-reply-2", "segment-1", true},
		{"user", "deepseek target turn gamma", targetSegmentID, false},
		{"loom", "loom-native-deepseek-reply-1", targetSegmentID, true},
		{"user", "deepseek target turn delta", targetSegmentID, false},
		{"loom", "loom-native-deepseek-reply-2", targetSegmentID, true},
	}
	seen := make(map[string]struct{}, len(thread.Messages))
	for index, message := range thread.Messages {
		if index >= len(want) || message.MessageID == "" || message.CreatedAt.IsZero() ||
			message.Role != want[index].role || message.Content != want[index].content ||
			message.SegmentID != want[index].segmentID || message.Tentative != want[index].tentative {
			t.Fatalf("message %d = %#v, want %#v", index, message, want[index])
		}
		if _, duplicate := seen[message.MessageID]; duplicate {
			t.Fatalf("duplicate message ID %q", message.MessageID)
		}
		seen[message.MessageID] = struct{}{}
	}
}

func productGatewaySegmentAcceptanceAssertSessionsAndEvents(
	t *testing.T,
	events []harnessgateway.Event,
) {
	t.Helper()
	var openings []harnessgateway.Event
	for _, event := range events {
		if event.Type == harnessgateway.EventSessionOpening {
			openings = append(openings, event)
		}
	}
	if len(openings) != 2 {
		t.Fatalf("Gateway Session openings = %#v", openings)
	}
	bySegment := make(map[string]harnessgateway.Event, len(openings))
	for _, event := range openings {
		bySegment[event.SegmentID] = event
	}
	codex := bySegment["segment-1"]
	native := bySegment["segment-2"]
	if codex.ConversationID != productGatewaySegmentAcceptanceThreadID ||
		codex.HarnessID != harnessgateway.HarnessCodex ||
		codex.BackendID != productCodexSegmentBackendID ||
		native.ConversationID != productGatewaySegmentAcceptanceThreadID ||
		native.HarnessID != harnessgateway.HarnessLoomNative ||
		native.BackendID != harnessgateway.BackendID("backend.segment.loom-native") ||
		codex.RouteTransitionReviewDigest != "" ||
		!validProductHex(native.RouteTransitionReviewDigest, 64) ||
		codex.SessionID == "" || native.SessionID == "" || codex.SessionID == native.SessionID {
		t.Fatalf("Gateway Session transition: Codex=%#v LoomNative=%#v", codex, native)
	}
	codexCompleted := 0
	for _, event := range events {
		if event.Type == harnessgateway.EventResponseCompleted && event.SessionID == codex.SessionID {
			codexCompleted++
		}
	}
	if codexCompleted != 2 {
		t.Fatalf("Codex completed responses = %d, want 2", codexCompleted)
	}
	nativeCompleted := 0
	for _, event := range events {
		if event.Type == harnessgateway.EventResponseCompleted && event.SessionID == native.SessionID {
			nativeCompleted++
		}
	}
	if nativeCompleted != 2 {
		t.Fatalf("Loom Native completed responses = %d, want 2", nativeCompleted)
	}
	productGatewaySegmentAcceptanceAssertContentFreeEvents(t, events)
}

func productGatewaySegmentAcceptanceAssertContentFreeEvents(
	t *testing.T,
	events []harnessgateway.Event,
) {
	t.Helper()
	if len(events) == 0 || events[0].GatewayInstanceID == "" {
		t.Fatal("Gateway event stream has no instance authority")
	}
	instanceID := events[0].GatewayInstanceID
	for index, event := range events {
		if !event.Valid() || event.WorkspacePath != "" || event.Content != "" ||
			event.ProviderBody != "" || event.GatewayInstanceID != instanceID {
			t.Fatalf("Gateway event %d contains invalid or content-bearing data: %#v", index, event)
		}
		encoded, err := json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{
			"codex source turn alpha", "codex source turn beta",
			"deepseek target turn gamma", "deepseek target turn delta", "codex-runtime-reply",
			"loom-native-deepseek-reply", "overlap private turn",
		} {
			if strings.Contains(string(encoded), forbidden) {
				t.Fatalf("Gateway event %d exposed content %q: %s", index, forbidden, encoded)
			}
		}
	}
}

func productGatewaySegmentAcceptanceCodexBinding() api.LocalProductConversationExecutionBinding {
	return api.LocalProductConversationExecutionBinding{
		SchemaVersion: 4, HarnessAdapter: "codex", ProviderID: "openai",
		ProviderAccountID: "", CredentialRevision: 0,
		ModelID: productGatewaySegmentAcceptanceCodexModel,
	}
}

func productGatewaySegmentAcceptanceNativeBinding() api.LocalProductConversationExecutionBinding {
	return api.LocalProductConversationExecutionBinding{
		SchemaVersion: 4, HarnessAdapter: "loom-native", ProviderID: "deepseek",
		ProviderAccountID: "deepseek.primary", CredentialRevision: 7,
		ModelID:                      productGatewaySegmentAcceptanceNativeModel,
		ProviderAccountPolicyVersion: 2, ProviderAccountPolicyRevision: 13,
		ProviderAccountPolicyDigest: strings.Repeat("d", 64),
		TrustDomain:                 "external_provider", RetentionMode: "limited_retention",
		DataRegion: "apac",
	}
}

func productGatewaySegmentAcceptanceClock() time.Time {
	return time.Unix(1_800_000_000, 123).UTC()
}

type productGatewaySegmentAcceptanceBindingResolver struct {
	bindings map[string]api.LocalProductConversationExecutionBinding
}

func (resolver *productGatewaySegmentAcceptanceBindingResolver) ResolveConversationExecutionBinding(
	_ context.Context,
	profileID string,
	modelID string,
) (api.LocalProductConversationExecutionBinding, error) {
	binding, ok := resolver.bindings[profileID]
	if !ok || binding.ModelID != modelID {
		return api.LocalProductConversationExecutionBinding{}, api.ErrInvalidLocalProductChatRequest
	}
	return binding, nil
}

func (resolver *productGatewaySegmentAcceptanceBindingResolver) ResolveConversationContextTarget(
	_ context.Context,
	threadID string,
	segmentID string,
	profileID string,
	modelID string,
) (contextcapsule.Target, error) {
	binding, ok := resolver.bindings[profileID]
	if !ok || binding.ModelID != modelID {
		return contextcapsule.Target{}, api.ErrInvalidLocalProductChatRequest
	}
	return contextcapsule.Target{
		ConversationID: threadID, TeamID: "conversation:" + threadID,
		AgentID: "conversation-agent:loom", RoleID: segmentID,
		ProviderID: binding.ProviderID, ProviderAccountID: binding.ProviderAccountID,
		ModelID: binding.ModelID, AuthMode: "native_auth",
		ContextAdapterID:        "context:" + binding.HarnessAdapter + ":v1",
		DisclosurePolicyID:      "loom.local-conversation-disclosure",
		DisclosurePolicyVersion: 1, TokenBudget: 8_192,
	}, nil
}

func (*productGatewaySegmentAcceptanceBindingResolver) ResolveConversationContextCapacity(
	_ context.Context,
	_ contextcapsule.Target,
) (contextcapsule.CapacityAuthority, contextcapsule.TokenCounter, error) {
	return contextcapsule.CapacityAuthority{
		SchemaVersion:       contextcapsule.CapacitySchemaVersion,
		Status:              contextcapsule.CapacityUnavailable,
		TokenCounterID:      productConversationCapacityCounter.ID(),
		TokenCounterVersion: productConversationCapacityCounter.Version(),
	}, productConversationCapacityCounter, nil
}

type productGatewaySegmentAcceptanceCapsuleStore struct {
	mu       sync.Mutex
	capsules map[string]contextcapsule.RoleContextCapsule
}

func (store *productGatewaySegmentAcceptanceCapsuleStore) PutRoleContextCapsule(
	_ context.Context,
	capsule contextcapsule.RoleContextCapsule,
	_ []byte,
) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.capsules == nil {
		store.capsules = make(map[string]contextcapsule.RoleContextCapsule)
	}
	authority := capsule.AuthorityRecord()
	store.capsules[authority.ConversationID+"\x00"+authority.RoleID] = capsule
	return nil
}

func (store *productGatewaySegmentAcceptanceCapsuleStore) DeleteRoleContextCapsule(
	_ context.Context,
	authority contextcapsule.AuthorityRecord,
) error {
	store.mu.Lock()
	delete(store.capsules, authority.ConversationID+"\x00"+authority.RoleID)
	store.mu.Unlock()
	return nil
}

func (store *productGatewaySegmentAcceptanceCapsuleStore) DeleteContextConversation(
	_ context.Context,
	conversationID string,
) error {
	store.mu.Lock()
	for key := range store.capsules {
		if strings.HasPrefix(key, conversationID+"\x00") {
			delete(store.capsules, key)
		}
	}
	store.mu.Unlock()
	return nil
}

type productGatewaySegmentAcceptanceExecutor struct {
	mu       sync.Mutex
	requests []api.LocalProductConversationRequest
}

func (executor *productGatewaySegmentAcceptanceExecutor) Respond(
	_ context.Context,
	request api.LocalProductConversationRequest,
) (api.LocalProductConversationResponse, error) {
	executor.mu.Lock()
	executor.requests = append(executor.requests, request)
	count := len(executor.requests)
	executor.mu.Unlock()
	return api.LocalProductConversationResponse{
		Content: fmt.Sprintf("loom-native-deepseek-reply-%d", count), Tentative: true,
	}, nil
}

func (executor *productGatewaySegmentAcceptanceExecutor) requestsSnapshot() []api.LocalProductConversationRequest {
	executor.mu.Lock()
	defer executor.mu.Unlock()
	return append([]api.LocalProductConversationRequest(nil), executor.requests...)
}

type productGatewaySegmentAcceptanceOverlapExecutor struct {
	mu          sync.Mutex
	started     int
	bothStarted chan struct{}
	release     chan struct{}
	once        sync.Once
}

func newProductGatewaySegmentAcceptanceOverlapExecutor() *productGatewaySegmentAcceptanceOverlapExecutor {
	return &productGatewaySegmentAcceptanceOverlapExecutor{
		bothStarted: make(chan struct{}), release: make(chan struct{}),
	}
}

func (executor *productGatewaySegmentAcceptanceOverlapExecutor) Respond(
	ctx context.Context,
	request api.LocalProductConversationRequest,
) (api.LocalProductConversationResponse, error) {
	executor.mu.Lock()
	executor.started++
	if executor.started == 2 {
		executor.once.Do(func() { close(executor.bothStarted) })
	}
	executor.mu.Unlock()
	select {
	case <-executor.release:
		return api.LocalProductConversationResponse{
			Content: "overlap-reply:" + request.ThreadID, Tentative: true,
		}, nil
	case <-ctx.Done():
		return api.LocalProductConversationResponse{}, ctx.Err()
	}
}

type productGatewaySegmentAcceptanceEventSink struct {
	mu     sync.Mutex
	events []harnessgateway.Event
}

func (sink *productGatewaySegmentAcceptanceEventSink) Record(event harnessgateway.Event) {
	sink.mu.Lock()
	sink.events = append(sink.events, event)
	sink.mu.Unlock()
}

func (sink *productGatewaySegmentAcceptanceEventSink) snapshot() []harnessgateway.Event {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	return append([]harnessgateway.Event(nil), sink.events...)
}

type productGatewaySegmentAcceptanceCodexRuntime struct {
	mu      sync.Mutex
	prompts []string
	closed  bool
}

func (runtime *productGatewaySegmentAcceptanceCodexRuntime) Respond(
	_ context.Context,
	prompt []byte,
) (harnessadapter.HarnessProcessResult, error) {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	runtime.prompts = append(runtime.prompts, string(prompt))
	return harnessadapter.HarnessProcessResult{
		Content: fmt.Sprintf("codex-runtime-reply-%d", len(runtime.prompts)),
	}, nil
}

func (runtime *productGatewaySegmentAcceptanceCodexRuntime) Healthy() bool {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return !runtime.closed
}

func (runtime *productGatewaySegmentAcceptanceCodexRuntime) Close(context.Context) error {
	runtime.mu.Lock()
	runtime.closed = true
	runtime.mu.Unlock()
	return nil
}

func (runtime *productGatewaySegmentAcceptanceCodexRuntime) promptsSnapshot() []string {
	runtime.mu.Lock()
	defer runtime.mu.Unlock()
	return append([]string(nil), runtime.prompts...)
}

type productGatewaySegmentAcceptanceSessionRunner struct{}

func (productGatewaySegmentAcceptanceSessionRunner) StartSession(
	context.Context,
	harnessadapter.HarnessSessionRequest,
) (harnessadapter.HarnessStreamSession, error) {
	return nil, nil
}
