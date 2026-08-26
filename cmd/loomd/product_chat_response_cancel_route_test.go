package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/harnessgateway"
	"loom-pi-rebuild/internal/localipc"
)

func TestProductDaemonRoutesMetadataOnlyChatResponseCancel(t *testing.T) {
	canceller := &productChatResponseCancellerFixture{}
	handler := newProductRouteHandler(productRouteServices{chatCancel: canceller})
	response := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "cancel-control-1", Method: "chat_response_cancel",
		Params: json.RawMessage(`{"thread_id":"thread-1","incident_id":"incident-send-1"}`),
	})
	if !response.OK || response.Error != nil || len(canceller.requests) != 1 ||
		canceller.requests[0].ThreadID != "thread-1" ||
		canceller.requests[0].IncidentID != "incident-send-1" {
		t.Fatalf("response=%#v requests=%#v", response, canceller.requests)
	}
	var acknowledgement struct {
		ThreadID   string `json:"thread_id"`
		IncidentID string `json:"incident_id"`
		Cancelled  bool   `json:"cancelled"`
	}
	if err := json.Unmarshal(response.Result, &acknowledgement); err != nil ||
		acknowledgement.ThreadID != "thread-1" ||
		acknowledgement.IncidentID != "incident-send-1" || !acknowledgement.Cancelled {
		t.Fatalf("acknowledgement=%#v error=%v", acknowledgement, err)
	}

	invalid := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "cancel-control-invalid", Method: "chat_response_cancel",
		Params: json.RawMessage(`{"thread_id":"thread-1","incident_id":"incident-send-1","content":"forbidden"}`),
	})
	if invalid.OK || invalid.Error == nil || invalid.Error.Code != "invalid_request" ||
		len(canceller.requests) != 1 {
		t.Fatalf("invalid=%#v requests=%#v", invalid, canceller.requests)
	}
}

func TestProductDaemonChatResponseCancelReturnsNotFoundForWrongIncident(t *testing.T) {
	canceller := &productChatResponseCancellerFixture{err: harnessgateway.ErrResponseNotFound}
	response := newProductRouteHandler(productRouteServices{chatCancel: canceller})(
		context.Background(),
		localipc.Request{
			Version: 1, RequestID: "cancel-control-not-found", Method: "chat_response_cancel",
			Params: json.RawMessage(`{"thread_id":"thread-1","incident_id":"incident-wrong"}`),
		},
	)
	if response.OK || response.Error == nil || response.Error.Code != "not_found" {
		t.Fatalf("response=%#v", response)
	}
}

func TestProductDaemonCancelsChatResponseThroughIndependentPrivateUDSRequest(t *testing.T) {
	route := newProductChatResponseCancelUDSFixture()
	root, err := os.MkdirTemp("/private/tmp", "loom-chat-cancel-route-")
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
		BuildID: "phase2d-chat-response-cancel",
		Handler: localipc.HandlerFunc(newProductRouteHandler(productRouteServices{
			read: route, chatCancel: route,
		})),
	})
	if err != nil {
		t.Fatal(err)
	}
	serverContext, stopServer := context.WithCancel(context.Background())
	serverDone := make(chan error, 1)
	go func() { serverDone <- server.Serve(serverContext) }()
	select {
	case <-server.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("private UDS did not become ready")
	}
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath, Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	sendResult := make(chan api.LocalProductChatThread, 1)
	sendError := make(chan error, 1)
	go func() {
		var thread api.LocalProductChatThread
		err := client.Call(context.Background(), "chat_message", map[string]any{
			"thread_id": "thread-private-cancel", "content": "stop this response",
			"profile_id": "profile-1", "context_mode": "start_clean",
		}, &thread)
		sendResult <- thread
		sendError <- err
	}()
	select {
	case <-route.started:
	case <-time.After(5 * time.Second):
		t.Fatal("chat response did not start")
	}
	var acknowledgement struct {
		ThreadID   string `json:"thread_id"`
		IncidentID string `json:"incident_id"`
		Cancelled  bool   `json:"cancelled"`
	}
	if err := client.Call(context.Background(), "chat_response_cancel", map[string]any{
		"thread_id": "thread-private-cancel", "incident_id": route.incidentID(),
	}, &acknowledgement); err != nil {
		t.Fatal(err)
	}
	if !acknowledgement.Cancelled || acknowledgement.ThreadID != "thread-private-cancel" {
		t.Fatalf("acknowledgement=%#v", acknowledgement)
	}
	thread := <-sendResult
	if err := <-sendError; err != nil || len(thread.Attempts) != 1 ||
		thread.Attempts[0].Status != "cancelled" ||
		thread.Attempts[0].IncidentID != acknowledgement.IncidentID {
		t.Fatalf("thread=%#v error=%v", thread, err)
	}
	stopServer()
	select {
	case err := <-serverDone:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("private UDS did not stop")
	}
}

type productChatResponseCancellerFixture struct {
	requests []api.LocalProductChatResponseCancelRequest
	err      error
}

func (fixture *productChatResponseCancellerFixture) CancelChatResponse(
	_ context.Context,
	request api.LocalProductChatResponseCancelRequest,
) error {
	fixture.requests = append(fixture.requests, request)
	return fixture.err
}

var _ api.LocalProductConversationResponseCanceller = (*productChatResponseCancellerFixture)(nil)

type productChatResponseCancelUDSFixture struct {
	productReadRoute
	started  chan struct{}
	stopped  chan struct{}
	once     sync.Once
	mu       sync.Mutex
	incident string
}

func newProductChatResponseCancelUDSFixture() *productChatResponseCancelUDSFixture {
	return &productChatResponseCancelUDSFixture{
		started: make(chan struct{}), stopped: make(chan struct{}),
	}
}

func (fixture *productChatResponseCancelUDSFixture) SendChatMessage(
	_ context.Context,
	request api.LocalProductChatMessageRequest,
) (api.LocalProductChatThread, error) {
	fixture.mu.Lock()
	fixture.incident = request.IncidentID
	fixture.mu.Unlock()
	close(fixture.started)
	<-fixture.stopped
	return api.LocalProductChatThread{
		ThreadID: request.ThreadID,
		Attempts: []api.LocalProductConversationAttempt{{
			AttemptID: "attempt-1", SegmentID: "segment-1",
			IncidentID: request.IncidentID, Status: "cancelled",
			CompletedAt: time.Unix(20, 0).UTC(),
		}},
	}, nil
}

func (fixture *productChatResponseCancelUDSFixture) CancelChatResponse(
	_ context.Context,
	request api.LocalProductChatResponseCancelRequest,
) error {
	if request.IncidentID != fixture.incidentID() {
		return harnessgateway.ErrResponseNotFound
	}
	fixture.once.Do(func() { close(fixture.stopped) })
	return nil
}

func (fixture *productChatResponseCancelUDSFixture) incidentID() string {
	fixture.mu.Lock()
	defer fixture.mu.Unlock()
	return fixture.incident
}
