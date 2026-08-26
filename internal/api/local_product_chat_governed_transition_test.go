package api

import (
	"context"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/contextcapsule"
)

type governedTransitionCapacityResolver struct {
	targets   map[string]contextcapsule.Target
	authority contextcapsule.CapacityAuthority
	counter   contextcapsule.TokenCounter
}

func (resolver governedTransitionCapacityResolver) ResolveConversationContextTarget(
	_ context.Context,
	threadID string,
	segmentID string,
	profileID string,
	modelID string,
) (contextcapsule.Target, error) {
	target, ok := resolver.targets[profileID]
	if !ok {
		return contextcapsule.Target{}, ErrInvalidLocalProductChatRequest
	}
	target.ConversationID = threadID
	target.TeamID = "conversation:" + threadID
	target.AgentID = "conversation-agent"
	target.RoleID = segmentID
	if modelID != "" {
		target.ModelID = modelID
	}
	return target, nil
}

func (resolver governedTransitionCapacityResolver) ResolveConversationContextCapacity(
	_ context.Context,
	_ contextcapsule.Target,
) (contextcapsule.CapacityAuthority, contextcapsule.TokenCounter, error) {
	return resolver.authority, resolver.counter, nil
}

func TestGovernedRouteTransitionFreezesTrustReviewCapacityAndCapsuleAsOneAuthority(
	t *testing.T,
) {
	const (
		threadID      = "governed-transition"
		sourceProfile = "conversation-deepseek-work-r7"
		targetProfile = "conversation-anthropic-work-r5"
	)
	sourceBinding := LocalProductConversationExecutionBinding{
		SchemaVersion: 4, HarnessAdapter: "loom-native",
		ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
		CredentialRevision: 7, ModelID: "deepseek-chat",
		ProviderAccountPolicyVersion: 2, ProviderAccountPolicyRevision: 3,
		ProviderAccountPolicyDigest: strings.Repeat("a", 64),
		TrustDomain:                 "external_provider", RetentionMode: "provider_default",
		DataRegion: "global",
	}
	targetBinding := LocalProductConversationExecutionBinding{
		SchemaVersion: 4, HarnessAdapter: "claude-code",
		ProviderID: "anthropic", ProviderAccountID: "anthropic.work",
		CredentialRevision: 5, ModelID: "claude-sonnet-5",
		ProviderAccountPolicyVersion: 2, ProviderAccountPolicyRevision: 4,
		ProviderAccountPolicyDigest: strings.Repeat("b", 64),
		TrustDomain:                 "enterprise_tenant", RetentionMode: "zero_data_retention",
		DataRegion: "apac",
	}
	counter := &scriptedConversationTokenCounter{id: "governed-counter", version: "v1"}
	resolver := governedTransitionCapacityResolver{
		targets: map[string]contextcapsule.Target{
			sourceProfile: {
				ProviderID: "deepseek", ProviderAccountID: "deepseek.work",
				ModelID: "deepseek-chat", AuthMode: "brokered",
				ContextAdapterID:   "context:loom-native:v1",
				DisclosurePolicyID: "test-disclosure", DisclosurePolicyVersion: 1,
				TokenBudget: 256,
			},
			targetProfile: {
				ProviderID: "anthropic", ProviderAccountID: "anthropic.work",
				ModelID: "claude-sonnet-5", AuthMode: "brokered",
				ContextAdapterID:   "context:claude-code:v1",
				DisclosurePolicyID: "test-disclosure", DisclosurePolicyVersion: 1,
				TokenBudget: 256,
			},
		},
		authority: contextcapsule.CapacityAuthority{
			SchemaVersion: contextcapsule.CapacitySchemaVersion,
			Status:        contextcapsule.CapacityExact, ContextWindowTokens: 512,
			ReservedOutputTokens: 64, AdapterToolOverheadTokens: 32,
			TokenCounterID: counter.ID(), TokenCounterVersion: counter.Version(),
		},
		counter: counter,
	}
	store := &recordingLocalProductConversationCapsuleStore{}
	requests := make([]LocalProductConversationRequest, 0, 2)
	chat := NewLocalProductChatAPI(func() time.Time { return time.Unix(1, 0).UTC() })
	chat.bindingResolver = localProductConversationBindingResolverFunc(func(
		_ context.Context,
		profileID string,
	) (LocalProductConversationExecutionBinding, error) {
		switch profileID {
		case sourceProfile:
			return sourceBinding, nil
		case targetProfile:
			return targetBinding, nil
		default:
			return LocalProductConversationExecutionBinding{}, ErrInvalidLocalProductChatRequest
		}
	})
	chat.responder = localProductConversationResponderFunc(func(
		_ context.Context,
		request LocalProductConversationRequest,
	) (LocalProductConversationResponse, error) {
		requests = append(requests, request)
		return LocalProductConversationResponse{Content: "accepted"}, nil
	})
	if err := chat.SetConversationContextCapsuleRuntime(resolver, store); err != nil {
		t.Fatal(err)
	}

	source, err := chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
		ThreadID: threadID, ProfileID: sourceProfile, Content: "authoritative goal",
	})
	if err != nil || len(source.Segments) != 1 || len(source.Attempts) != 1 {
		t.Fatalf("source thread=%#v error=%v", source, err)
	}
	review, err := NewLocalProductConversationTrustBoundaryAcknowledgement(
		threadID, source.Segments[0], targetProfile, targetBinding, "", ContextModeSummaryOnly,
	)
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
		ThreadID: threadID, ProfileID: targetProfile, ModelID: targetBinding.ModelID,
		Content: "continue under reviewed target", ContextMode: ContextModeSummaryOnly,
		ExpectedExecutionBinding: &targetBinding, TrustBoundaryAcknowledgement: &review,
	})
	if err != nil || len(accepted.Segments) != 2 || len(accepted.Attempts) != 2 ||
		len(requests) != 2 || len(store.records) != 2 {
		t.Fatalf("accepted thread=%#v requests=%d records=%d error=%v", accepted, len(requests), len(store.records), err)
	}
	segment := accepted.Segments[1]
	attempt := accepted.Attempts[1]
	dispatch := requests[1]
	if segment.ExecutionBinding == nil || *segment.ExecutionBinding != targetBinding ||
		attempt.ExecutionBinding == nil || *attempt.ExecutionBinding != targetBinding ||
		segment.RouteTransitionReviewDigest != review.ReviewDigest ||
		attempt.RouteTransitionReviewDigest != review.ReviewDigest ||
		segment.ContextCapsuleDigest == "" ||
		segment.ContextCapsuleDigest != attempt.ContextCapsuleDigest ||
		segment.ContextCapsuleDigest != dispatch.ContextCapsuleDigest ||
		segment.DisclosureReceiptDigest != attempt.DisclosureReceiptDigest ||
		segment.DisclosureReceiptDigest != dispatch.DisclosureReceiptDigest ||
		segment.ContextCapacityStatus != contextcapsule.CapacityExact ||
		segment.ContextWindowTokens != 512 || segment.ReservedOutputTokens != 64 ||
		segment.AdapterToolOverheadTokens != 32 ||
		segment.AdmittedInputBudgetTokens != 256 ||
		attempt.ContextCapacityStatus != segment.ContextCapacityStatus ||
		attempt.AdmittedInputBudgetTokens != segment.AdmittedInputBudgetTokens ||
		dispatch.ContextCapacityStatus != segment.ContextCapacityStatus ||
		dispatch.AdmittedInputBudgetTokens != segment.AdmittedInputBudgetTokens ||
		segment.BindingDigest == "" || attempt.BindingDigest != segment.BindingDigest ||
		dispatch.BindingDigest != segment.BindingDigest ||
		dispatch.SegmentBindingDigest != segment.BindingDigest {
		t.Fatalf("governed authority diverged: segment=%#v attempt=%#v dispatch=%#v", segment, attempt, dispatch)
	}
	if err := validateStoredChatThread(accepted); err != nil {
		t.Fatalf("combined authority did not survive validation: %v", err)
	}

	legacy := cloneChatThread(&accepted)
	legacyReviewDigest := legacyLocalProductTrustBoundaryReviewDigestV1(
		legacy.ThreadID,
		legacy.Segments[0],
		*legacy.Segments[1].ExecutionBinding,
		legacy.Segments[1].ContextMode,
	)
	legacy.Segments[1].RouteTransitionReviewDigest = legacyReviewDigest
	legacyDisclosure := conversationContextDisclosureFromSegment(legacy.Segments[1])
	legacyBindingDigest := conversationExecutionBindingDigestWithRouteReview(
		legacy.Segments[1].SegmentID,
		legacy.Segments[1].ProfileID,
		legacy.Segments[1].ModelID,
		legacy.Segments[1].ReasoningEffort,
		legacy.Segments[1].ContextMode,
		legacyDisclosure,
		legacy.Segments[1].ExecutionBinding,
		legacyReviewDigest,
	)
	legacy.Segments[1].BindingDigest = legacyBindingDigest
	legacy.Attempts[1].RouteTransitionReviewDigest = legacyReviewDigest
	legacy.Attempts[1].BindingDigest = legacyBindingDigest
	if err := validateStoredChatThread(*legacy); err != nil {
		t.Fatalf("Build 127 v1 review authority did not remain readable: %v", err)
	}
}
