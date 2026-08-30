package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/localipc"
)

func TestProductChatMessageRoutePreservesSessionCatalog(t *testing.T) {
	route := &productChatMessageRouteFixture{}
	handler := newProductRouteHandler(productRouteServices{read: route})
	response := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "incident-session-catalog-1", Method: "chat_message",
		Params: json.RawMessage(`{
			"thread_id":"p7-capability-sweep-1",
			"content":"Use the available Loom capability.",
			"profile_id":"conversation-openai-codex-default-v1",
			"model_id":"gpt-5.6-sol",
			"context_mode":"start_clean",
			"session_catalog":[
				{"conversation_id":"p7-source-alpha","title":"P7 source alpha","updated_at":"2026-08-29T09:30:00Z"},
				{"conversation_id":"p7-source-beta","title":"P7 source beta","updated_at":"2026-08-29T09:30:01Z"},
				{"conversation_id":"p7-capability-sweep-1","title":"P7 capability sweep reads-foundation","updated_at":"2026-08-29T09:30:02Z"}
			]
		}`),
	})
	if !response.OK || response.Error != nil || !route.called {
		t.Fatalf("response = %#v called = %t", response, route.called)
	}
	request := route.request
	if request.IncidentID != "incident-session-catalog-1" ||
		request.ThreadID != "p7-capability-sweep-1" || len(request.SessionCatalog) != 3 ||
		request.SessionCatalog[0].ConversationID != "p7-source-alpha" ||
		request.SessionCatalog[1].ConversationID != "p7-source-beta" ||
		request.SessionCatalog[2].ConversationID != request.ThreadID ||
		!request.SessionCatalog[2].UpdatedAt.Equal(time.Date(2026, 8, 29, 9, 30, 2, 0, time.UTC)) {
		t.Fatalf("request = %#v", request)
	}

	invalid := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "incident-session-catalog-2", Method: "chat_message",
		Params: json.RawMessage(`{
			"thread_id":"p7-capability-sweep-2","content":"Check Loom status.",
			"profile_id":"conversation-openai-codex-default-v1","context_mode":"start_clean",
			"session_catalog":[],"prompt":"must remain rejected"
		}`),
	})
	if invalid.OK || invalid.Error == nil || invalid.Error.Code != "invalid_request" {
		t.Fatalf("invalid response = %#v", invalid)
	}
}

func TestProductChatControlDecisionRoutePreservesIncidentAndMetadataOnly(t *testing.T) {
	controller := &productChatControlRouteFixture{}
	handler := newProductRouteHandler(productRouteServices{chatControl: controller})
	params := json.RawMessage(`{
		"thread_id":"s3",
		"proposal_id":"proposal-1",
		"proposal_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"decision":"confirm"
	}`)
	response := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "incident-control-1",
		Method: "chat_control_decision", Params: params,
	})
	if !response.OK || response.Error != nil {
		t.Fatalf("response = %#v", response)
	}
	if controller.request.ThreadID != "s3" ||
		controller.request.ProposalID != "proposal-1" ||
		controller.request.Decision != api.ControlDecisionConfirm ||
		controller.request.IncidentID != "incident-control-1" {
		t.Fatalf("request = %#v", controller.request)
	}
	encoded, err := json.Marshal(controller.request)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"content", "credential", "api_key", "prompt"} {
		if json.Valid(encoded) && containsJSONField(encoded, forbidden) {
			t.Fatalf("decision request leaked %q: %s", forbidden, encoded)
		}
	}
}

func TestProductChatControlDecisionRouteRejectsUnknownFields(t *testing.T) {
	controller := &productChatControlRouteFixture{}
	response := newProductRouteHandler(productRouteServices{chatControl: controller})(
		context.Background(),
		localipc.Request{
			Version: 1, RequestID: "incident-control-2",
			Method: "chat_control_decision",
			Params: json.RawMessage(`{
				"thread_id":"s3","proposal_id":"proposal-1",
				"proposal_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"decision":"confirm","content":"private"
			}`),
		},
	)
	if response.OK || response.Error == nil || response.Error.Code != "invalid_request" ||
		controller.called {
		t.Fatalf("response = %#v called=%t", response, controller.called)
	}
}

type productChatControlRouteFixture struct {
	request api.LocalProductChatControlDecisionRequest
	called  bool
}

type productChatMessageRouteFixture struct {
	productReadRoute
	request api.LocalProductChatMessageRequest
	called  bool
}

func (fixture *productChatMessageRouteFixture) SendChatMessage(
	_ context.Context,
	request api.LocalProductChatMessageRequest,
) (api.LocalProductChatThread, error) {
	fixture.called = true
	fixture.request = request
	return api.LocalProductChatThread{
		ThreadID: request.ThreadID, CanReply: true,
		Segments: []api.LocalProductConversationSegment{},
		Attempts: []api.LocalProductConversationAttempt{},
		Messages: []api.LocalProductChatMessage{},
	}, nil
}

var _ productReadRoute = (*productChatMessageRouteFixture)(nil)

func (fixture *productChatControlRouteFixture) DecideControlProposal(
	_ context.Context,
	request api.LocalProductChatControlDecisionRequest,
) (api.LocalProductChatThread, error) {
	fixture.called = true
	fixture.request = request
	return api.LocalProductChatThread{
		ThreadID: request.ThreadID, CanReply: true,
		Segments: []api.LocalProductConversationSegment{},
		Attempts: []api.LocalProductConversationAttempt{},
		Messages: []api.LocalProductChatMessage{},
	}, nil
}

func containsJSONField(body []byte, field string) bool {
	var object map[string]json.RawMessage
	if json.Unmarshal(body, &object) != nil {
		return false
	}
	_, found := object[field]
	return found
}
