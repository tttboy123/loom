package api

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/contextcapsule"
)

func TestChatContextDisclosureReturnsOnlyPrivacySafeMetadata(t *testing.T) {
	chat, store, thread := localProductContextDisclosureFixture(t)
	segment := thread.Segments[1]

	result, err := chat.InspectChatContextDisclosure(
		context.Background(),
		LocalProductChatContextDisclosureRequest{
			ThreadID: thread.ThreadID, SegmentID: segment.SegmentID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.SchemaVersion != 1 || result.ThreadID != thread.ThreadID ||
		result.SegmentID != segment.SegmentID ||
		result.ContextCapsuleDigest != segment.ContextCapsuleDigest ||
		result.DisclosureReceiptDigest != segment.DisclosureReceiptDigest ||
		len(result.Disclosed) != 2 || len(result.Omitted) != 1 {
		t.Fatalf("disclosure=%#v", result)
	}
	if len(store.readAuthorities) != 1 ||
		store.readAuthorities[0].RoleID != segment.SegmentID ||
		result.Disclosed[0].Kind != string(contextcapsule.KindRecentUserTurn) ||
		result.Disclosed[0].OmissionReason != "" ||
		result.Disclosed[0].Retrievable ||
		result.Omitted[0].Kind != string(contextcapsule.KindPriorModelOutput) ||
		result.Omitted[0].Trust != string(contextcapsule.TrustUntrusted) ||
		result.Omitted[0].Scope != string(contextcapsule.ScopeConversationShared) ||
		result.Omitted[0].OmissionReason != string(contextcapsule.OmissionPolicyFiltered) ||
		result.Omitted[0].Retrievable {
		t.Fatalf("metadata=%#v reads=%#v", result, store.readAuthorities)
	}

	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	assertContextDisclosureJSONKeys(t, encoded)
	for _, forbidden := range []string{
		"CONTENT_SENTINEL", "SOURCE_REF_SENTINEL", `"content":`, `"content_digest":`,
		`"source_ref":`, `"reference_id":`, `"prompt":`, `"provider_body":`, `"secret":`,
		`"dispatch_payload":`, `"hidden_reasoning":`, `"item_id":`, `"required":`,
	} {
		if strings.Contains(strings.ToLower(string(encoded)), strings.ToLower(forbidden)) {
			t.Fatalf("privacy-safe result contains %q: %s", forbidden, encoded)
		}
	}
}

func TestChatContextDisclosureFailsClosedWhenVaultReadReturnsDifferentCapsule(t *testing.T) {
	chat, store, thread := localProductContextDisclosureFixture(t)
	segment := thread.Segments[1]
	store.readCapsuleOverride = &store.records[0].capsule

	_, err := chat.InspectChatContextDisclosure(
		context.Background(),
		LocalProductChatContextDisclosureRequest{
			ThreadID: thread.ThreadID, SegmentID: segment.SegmentID,
		},
	)
	if !errors.Is(err, ErrLocalProductChatUnavailable) || len(store.readAuthorities) != 1 {
		t.Fatalf("err=%v reads=%#v", err, store.readAuthorities)
	}
}

func TestChatContextDisclosureFailsClosedOnVaultAuthorityMismatch(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(*contextcapsule.AuthorityRecord)
	}{
		{name: "role", mutate: func(authority *contextcapsule.AuthorityRecord) {
			authority.RoleID = "segment-forged"
		}},
		{name: "receipt", mutate: func(authority *contextcapsule.AuthorityRecord) {
			authority.DisclosureReceiptDigest = strings.Repeat("a", 64)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			chat, store, thread := localProductContextDisclosureFixture(t)
			segment := thread.Segments[1]
			authority := store.records[1].capsule.AuthorityRecord()
			test.mutate(&authority)
			store.authoritiesOverride = []contextcapsule.AuthorityRecord{authority}

			_, err := chat.InspectChatContextDisclosure(
				context.Background(),
				LocalProductChatContextDisclosureRequest{
					ThreadID: thread.ThreadID, SegmentID: segment.SegmentID,
				},
			)
			if !errors.Is(err, ErrLocalProductChatUnavailable) || len(store.readAuthorities) != 0 {
				t.Fatalf("err=%v reads=%#v", err, store.readAuthorities)
			}
		})
	}
}

func TestChatContextDisclosureMarksOnlyStoredBudgetOmissionsRetrievable(t *testing.T) {
	chat, _, thread := localProductContextDisclosureFixtureWith(
		t, ContextModeContinueWithContext, 3,
	)
	result, err := chat.InspectChatContextDisclosure(
		context.Background(),
		LocalProductChatContextDisclosureRequest{
			ThreadID: thread.ThreadID, SegmentID: thread.Segments[1].SegmentID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Disclosed) != 1 || len(result.Omitted) != 2 {
		t.Fatalf("disclosure=%#v", result)
	}
	for _, item := range result.Omitted {
		if item.OmissionReason != string(contextcapsule.OmissionBudgetExceeded) ||
			!item.Retrievable {
			t.Fatalf("omitted item=%#v", item)
		}
	}
}

func TestLocalProductReadServiceReusesOptionalChatContextDisclosureInspector(t *testing.T) {
	chat, _, thread := localProductContextDisclosureFixture(t)
	service := &LocalProductReadService{chat: chat}
	request := LocalProductChatContextDisclosureRequest{
		ThreadID: thread.ThreadID, SegmentID: thread.Segments[1].SegmentID,
	}
	result, err := service.ReadChatContextDisclosure(context.Background(), request)
	if err != nil || result.ContextCapsuleDigest != thread.Segments[1].ContextCapsuleDigest {
		t.Fatalf("result=%#v err=%v", result, err)
	}

	service.chat = localProductChatSourceWithoutDisclosureInspector{}
	if _, err := service.ReadChatContextDisclosure(context.Background(), request); !errors.Is(
		err, ErrLocalProductChatUnavailable,
	) {
		t.Fatalf("unsupported inspector err=%v", err)
	}
}

type localProductChatSourceWithoutDisclosureInspector struct{}

func (localProductChatSourceWithoutDisclosureInspector) ChatThread(
	context.Context, string,
) (LocalProductChatThread, error) {
	return LocalProductChatThread{}, nil
}

func (localProductChatSourceWithoutDisclosureInspector) SendMessage(
	context.Context, LocalProductChatMessageRequest,
) (LocalProductChatThread, error) {
	return LocalProductChatThread{}, nil
}

func (localProductChatSourceWithoutDisclosureInspector) DeleteThread(
	context.Context, string,
) error {
	return nil
}

func assertContextDisclosureJSONKeys(t *testing.T, encoded []byte) {
	t.Helper()
	var result map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &result); err != nil {
		t.Fatal(err)
	}
	topLevel := map[string]struct{}{
		"schema_version": {}, "thread_id": {}, "segment_id": {},
		"context_capsule_digest": {}, "disclosure_receipt_digest": {},
		"context_capacity_status": {}, "admitted_input_budget_tokens": {},
		"context_token_counter_id": {}, "context_token_counter_version": {},
		"admitted_contribution_tokens": {}, "context_capacity_contributions": {},
		"disclosed": {}, "omitted": {},
	}
	if len(result) != len(topLevel) {
		t.Fatalf("top-level keys=%v", result)
	}
	for key := range result {
		if _, allowed := topLevel[key]; !allowed {
			t.Fatalf("unexpected top-level key=%q", key)
		}
	}
	var disclosed []map[string]json.RawMessage
	if err := json.Unmarshal(result["disclosed"], &disclosed); err != nil || len(disclosed) == 0 {
		t.Fatalf("disclosed=%v err=%v", disclosed, err)
	}
	itemKeys := map[string]struct{}{
		"kind": {}, "trust": {}, "scope": {}, "token_count": {},
		"omission_reason": {}, "retrievable": {},
	}
	if len(disclosed[0]) != len(itemKeys) {
		t.Fatalf("item keys=%v", disclosed[0])
	}
	for key := range disclosed[0] {
		if _, allowed := itemKeys[key]; !allowed {
			t.Fatalf("unexpected item key=%q", key)
		}
	}
}

func localProductContextDisclosureFixture(
	t *testing.T,
) (*LocalProductChatAPI, *recordingLocalProductConversationCapsuleStore, LocalProductChatThread) {
	return localProductContextDisclosureFixtureWith(t, ContextModeSummaryOnly, 2_048)
}

func localProductContextDisclosureFixtureWith(
	t *testing.T,
	mode LocalProductContextMode,
	segmentTwoBudget int,
) (*LocalProductChatAPI, *recordingLocalProductConversationCapsuleStore, LocalProductChatThread) {
	t.Helper()
	chat := NewLocalProductChatAPI(func() time.Time { return time.Unix(1, 0).UTC() })
	chat.responder = localProductConversationResponderFunc(func(
		_ context.Context,
		_ LocalProductConversationRequest,
	) (LocalProductConversationResponse, error) {
		return LocalProductConversationResponse{
			Content: "CONTENT_SENTINEL model output", Tentative: true,
		}, nil
	})
	store := &recordingLocalProductConversationCapsuleStore{}
	if err := chat.SetConversationContextCapsuleRuntime(
		localProductConversationContextTargetResolverFunc(func(
			_ context.Context,
			threadID string,
			segmentID string,
			_ string,
		) (contextcapsule.Target, error) {
			tokenBudget := 2_048
			if segmentID == "segment-2" {
				tokenBudget = segmentTwoBudget
			}
			return contextcapsule.Target{
				ConversationID: threadID, TeamID: "conversation:" + threadID,
				AgentID: "conversation-agent", RoleID: segmentID,
				ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
				ModelID: "deepseek-chat", AuthMode: "brokered",
				ContextAdapterID:        "context:loom-native:v1",
				DisclosurePolicyID:      "loom.local-conversation-disclosure",
				DisclosurePolicyVersion: 1, TokenBudget: tokenBudget,
			}, nil
		}),
		store,
	); err != nil {
		t.Fatal(err)
	}
	const threadID = "context-disclosure-thread"
	const profileID = "conversation-deepseek-deepseek-chat-r3"
	if _, err := chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
		ThreadID: threadID, Content: "accepted goal", ProfileID: profileID,
	}); err != nil {
		t.Fatal(err)
	}
	thread, err := chat.SendMessage(context.Background(), LocalProductChatMessageRequest{
		ThreadID: threadID, Content: "next turn", ProfileID: profileID,
		ModelID: "deepseek-reasoner", ContextMode: mode,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(thread.Segments) != 2 || len(store.records) != 2 {
		t.Fatalf("thread=%#v records=%d", thread, len(store.records))
	}
	return chat, store, thread
}
