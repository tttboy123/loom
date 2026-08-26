package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/localipc"
)

type productContextDisclosureReadRoute struct {
	productReadRoute
	result   api.LocalProductChatContextDisclosure
	requests []api.LocalProductChatContextDisclosureRequest
}

type productContextDisclosureChatSource struct {
	api.LocalProductChatSource
	result api.LocalProductChatContextDisclosure
}

func (source *productContextDisclosureChatSource) InspectChatContextDisclosure(
	_ context.Context,
	_ api.LocalProductChatContextDisclosureRequest,
) (api.LocalProductChatContextDisclosure, error) {
	return source.result, nil
}

func (route *productContextDisclosureReadRoute) ReadChatContextDisclosure(
	_ context.Context,
	request api.LocalProductChatContextDisclosureRequest,
) (api.LocalProductChatContextDisclosure, error) {
	route.requests = append(route.requests, request)
	return route.result, nil
}

func TestProductDaemonRoutesPrivacySafeChatContextDisclosure(t *testing.T) {
	want := api.LocalProductChatContextDisclosure{
		SchemaVersion: 1, ThreadID: "thread-1", SegmentID: "segment-2",
		ContextCapsuleDigest:    productAgentInboxTestDigest("capsule"),
		DisclosureReceiptDigest: productAgentInboxTestDigest("receipt"),
		Disclosed: []api.LocalProductChatContextDisclosureItem{{
			Kind: "recent_user_turn", Trust: "authoritative",
			Scope: "conversation_shared", TokenCount: 4,
		}},
		Omitted: []api.LocalProductChatContextDisclosureItem{{
			Kind: "untrusted_model_output", Trust: "untrusted",
			Scope: "conversation_shared", TokenCount: 8,
			OmissionReason: "budget_exceeded", Retrievable: true,
		}},
	}
	route := &productContextDisclosureReadRoute{result: want}
	handler := newProductRouteHandler(productRouteServices{read: route})
	response := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "context-disclosure-1",
		Method: "chat_context_disclosure",
		Params: json.RawMessage(`{"thread_id":"thread-1","segment_id":"segment-2"}`),
	})
	if !response.OK || response.Error != nil || len(route.requests) != 1 ||
		route.requests[0].ThreadID != "thread-1" ||
		route.requests[0].SegmentID != "segment-2" {
		t.Fatalf("response=%#v requests=%#v", response, route.requests)
	}
	var got api.LocalProductChatContextDisclosure
	if err := json.Unmarshal(response.Result, &got); err != nil || got.ContextCapsuleDigest != want.ContextCapsuleDigest ||
		len(got.Omitted) != 1 || !got.Omitted[0].Retrievable {
		t.Fatalf("result=%#v err=%v", got, err)
	}

	invalid := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "context-disclosure-invalid",
		Method: "chat_context_disclosure",
		Params: json.RawMessage(`{"thread_id":"thread-1","segment_id":"segment-2","prompt":"forbidden"}`),
	})
	if invalid.OK || invalid.Error == nil || invalid.Error.Code != "invalid_request" ||
		len(route.requests) != 1 {
		t.Fatalf("invalid response=%#v requests=%#v", invalid, route.requests)
	}
}

func TestProductDaemonServesPrivacySafeChatContextDisclosureThroughPrivateUDS(t *testing.T) {
	want := api.LocalProductChatContextDisclosure{
		SchemaVersion: 1, ThreadID: "thread-private-1", SegmentID: "segment-private-2",
		ContextCapsuleDigest:    productAgentInboxTestDigest("private-capsule"),
		DisclosureReceiptDigest: productAgentInboxTestDigest("private-receipt"),
		ContextWindowTokens:     32_768, ReservedOutputTokens: 4_096,
		AdapterToolOverheadTokens: 512, AdmittedInputBudgetTokens: 28_160,
		AdmittedContributionTokens: 24, BudgetOmittedContributionTokens: 8,
		Disclosed: []api.LocalProductChatContextDisclosureItem{{
			Kind: "confirmed_user_constraint", Trust: "authoritative",
			Scope: "conversation_shared", TokenCount: 24,
		}},
		Omitted: []api.LocalProductChatContextDisclosureItem{{
			Kind: "untrusted_model_output", Trust: "untrusted",
			Scope: "conversation_shared", TokenCount: 8,
			OmissionReason: "budget_exceeded", Retrievable: true,
		}},
	}
	route := &productContextDisclosureReadRoute{result: want}
	root, err := os.MkdirTemp("/private/tmp", "loom-context-disclosure-route-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	socketPath := filepath.Join(root, "loomd.sock")
	server, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath: socketPath, EffectiveUID: os.Geteuid(),
		BuildID: "phase2d-context-disclosure-acceptance",
		Handler: localipc.HandlerFunc(newProductRouteHandler(productRouteServices{
			read: route,
		})),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	select {
	case <-server.Ready():
	case err := <-done:
		t.Fatalf("context disclosure server failed before ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("context disclosure server did not become ready")
	}
	t.Cleanup(func() {
		cancel()
		if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("context disclosure server close: %v", err)
		}
	})
	socketInfo, err := os.Lstat(socketPath)
	if err != nil || socketInfo.Mode().Perm() != 0o600 ||
		socketInfo.Mode()&os.ModeSocket == 0 {
		t.Fatalf("private IPC socket=%v err=%v", socketInfo, err)
	}
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath, Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	var got api.LocalProductChatContextDisclosure
	if err := client.Call(
		context.Background(), "chat_context_disclosure",
		api.LocalProductChatContextDisclosureRequest{
			ThreadID: want.ThreadID, SegmentID: want.SegmentID,
		},
		&got,
	); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) || len(route.requests) != 1 {
		t.Fatalf("disclosure=%#v requests=%#v", got, route.requests)
	}
	wire, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{
		"prompt", "content", "provider_body", "authorization", "credential",
		"private user context must never cross this route",
	} {
		if strings.Contains(strings.ToLower(string(wire)), forbidden) {
			t.Fatalf("private field %q crossed disclosure wire: %s", forbidden, wire)
		}
	}
	if got.DisclosureReceiptDigest == "" || got.ContextCapsuleDigest == "" ||
		len(got.Disclosed) != 1 || len(got.Omitted) != 1 || !got.Omitted[0].Retrievable {
		t.Fatalf("privacy-safe disclosure authority = %#v", got)
	}
}

func TestProductCompositionSlotsPreserveChatContextDisclosureCapability(t *testing.T) {
	want := api.LocalProductChatContextDisclosure{
		SchemaVersion: 1, ThreadID: "thread-1", SegmentID: "segment-1",
	}
	route := &productContextDisclosureReadRoute{result: want}
	readSlot := &productReadRouteSlot{}
	if err := readSlot.Bind(productReadRoutes{
		route: route, close: func() error { return nil },
	}); err != nil {
		t.Fatal(err)
	}
	got, err := readSlot.ReadChatContextDisclosure(
		context.Background(),
		api.LocalProductChatContextDisclosureRequest{
			ThreadID: "thread-1", SegmentID: "segment-1",
		},
	)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("read slot result=%#v err=%v", got, err)
	}

	conversationSlot := &productConversationRouteSlot{}
	conversationSlot.routes = productConversationRoutes{
		route: &productContextDisclosureChatSource{result: want},
		close: func() error { return nil },
	}
	conversationSlot.bound = true
	got, err = conversationSlot.InspectChatContextDisclosure(
		context.Background(),
		api.LocalProductChatContextDisclosureRequest{
			ThreadID: "thread-1", SegmentID: "segment-1",
		},
	)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("conversation slot result=%#v err=%v", got, err)
	}
}
