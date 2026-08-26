package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/harnessgateway"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/runtime/harnessadapter"
)

const (
	productHarnessGatewayCancelIPCThreadID = "conversation-codex-cancel-ipc"
	productHarnessGatewayCancelIPCProfile  = "conversation-openai-codex-cancel-ipc"
	productHarnessGatewayCancelIPCModel    = "gpt-5.6-codex"
	productHarnessGatewayCancelIPCIncident = "loom-client-1"
)

func TestProductHarnessGatewayCancelIPCSourceAcceptance(t *testing.T) {
	root, err := os.MkdirTemp("/private/tmp", "loom-harness-gateway-cancel-ipc-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(root, "workspace")
	privateRoot := filepath.Join(root, "codex-private")
	home := filepath.Join(root, "home")
	for _, directory := range []string{workspace, privateRoot, home} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	prepareProductHarnessGatewayCodexAuthFixture(t, home)

	appServer := newProductHarnessGatewayCancelIPCAppServer()
	sessions := &productHarnessGatewayCancelIPCSessionRunner{session: appServer}
	events := &productHarnessGatewayCancelIPCEventSink{}
	peer := &productHarnessGatewayCancelIPCPeerResponder{}
	responder, err := newProductHarnessGatewayConversationResponder(
		productHarnessGatewayConversationConfig{
			Executor: peer, WorkspacePath: workspace, Events: events,
			Now: func() time.Time { return time.Unix(1_800_000_100, 0).UTC() },
			CodexSegment: &productCodexSegmentBackendConfig{
				ExecutablePath: "/opt/loom/bin/codex", HomePath: home,
				PrivateRoot: privateRoot, Sessions: sessions,
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

	storePath := filepath.Join(root, "chat.json")
	chat, err := api.NewPersistentLocalProductChatAPI(
		storePath, func() time.Time { return time.Unix(1_800_000_200, 0).UTC() }, responder,
	)
	if err != nil {
		t.Fatal(err)
	}
	resolver := &productHarnessGatewayCancelIPCBindingResolver{bindings: map[string]api.LocalProductConversationExecutionBinding{
		productHarnessGatewayCancelIPCProfile: productHarnessGatewayCancelIPCCodexBinding(),
		"conversation-deepseek-peer":          productHarnessGatewayCancelIPCPeerBinding(),
	}}
	if err := chat.SetConversationExecutionBindingResolver(resolver); err != nil {
		t.Fatal(err)
	}
	route := &productHarnessGatewayCancelIPCRoute{chat: chat}

	socketPath := filepath.Join(root, "loomd.sock")
	server, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath: socketPath, EffectiveUID: os.Geteuid(),
		BuildID: "phase2d-hg1-cancel-source",
		Handler: localipc.HandlerFunc(newProductRouteHandler(productRouteServices{
			read: route, chatCancel: chat,
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
		t.Fatal("private IPC server did not become ready")
	}
	t.Cleanup(func() {
		stopServer()
		select {
		case serveErr := <-serverDone:
			if serveErr != nil && !errors.Is(serveErr, context.Canceled) {
				t.Errorf("close private IPC server: %v", serveErr)
			}
		case <-time.After(5 * time.Second):
			t.Error("private IPC server did not stop")
		}
	})
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath, Timeout: 10 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}

	cancelledResult := make(chan api.LocalProductChatThread, 1)
	cancelledError := make(chan error, 1)
	go func() {
		var thread api.LocalProductChatThread
		callErr := client.Call(context.Background(), "chat_message", map[string]any{
			"thread_id":    productHarnessGatewayCancelIPCThreadID,
			"content":      "hold this Codex response until I stop it",
			"profile_id":   productHarnessGatewayCancelIPCProfile,
			"model_id":     productHarnessGatewayCancelIPCModel,
			"context_mode": "start_clean",
		}, &thread)
		cancelledResult <- thread
		cancelledError <- callErr
	}()
	select {
	case <-appServer.firstTurnAccepted:
	case <-time.After(5 * time.Second):
		t.Fatal("Codex app-server did not accept turn/start")
	}
	if observable, cancelledRead := appServer.firstTurnResponseState(); observable || cancelledRead {
		t.Fatalf(
			"native turn ID response crossed the cancellation boundary early: observable=%t cancelled_read=%t",
			observable, cancelledRead,
		)
	}

	var peerThread api.LocalProductChatThread
	if err := client.Call(context.Background(), "chat_message", map[string]any{
		"thread_id": "conversation-peer-continues", "content": "answer while Codex is busy",
		"profile_id": "conversation-deepseek-peer", "model_id": "deepseek-chat",
		"context_mode": "start_clean",
	}, &peerThread); err != nil {
		t.Fatal(err)
	}
	if len(peerThread.Attempts) != 1 || peerThread.Attempts[0].Status != "succeeded" ||
		len(peerThread.Messages) != 2 || peerThread.Messages[1].Content != "peer conversation reply" ||
		peer.calls() != 1 {
		t.Fatalf("peer Conversation did not continue independently: %#v", peerThread)
	}
	peerRequests := peer.snapshot()
	if len(peerRequests) != 1 || peerRequests[0].ExecutionBinding == nil ||
		peerRequests[0].ExecutionBinding.ProviderAccountID != "" ||
		peerRequests[0].ExecutionBinding.CredentialRevision != 0 ||
		peerRequests[0].ExecutionBinding.ProviderAccountPolicyDigest != "" {
		t.Fatalf("Loom Native peer authority = %#v", peerRequests)
	}
	peerEvents := 0
	for _, event := range events.snapshot() {
		if event.ConversationID != "conversation-peer-continues" {
			continue
		}
		peerEvents++
		if event.ProviderAccountID != "" || event.CredentialRevision != 0 ||
			event.GovernancePolicyDigest != "" || event.RouteTransitionReviewDigest != "" {
			t.Fatalf("Loom Native peer Gateway authority = %#v", event)
		}
	}
	if peerEvents == 0 {
		t.Fatal("Loom Native peer emitted no Gateway events")
	}

	var acknowledgement struct {
		ThreadID   string `json:"thread_id"`
		IncidentID string `json:"incident_id"`
		Cancelled  bool   `json:"cancelled"`
	}
	if err := client.Call(context.Background(), "chat_response_cancel", map[string]any{
		"thread_id":   productHarnessGatewayCancelIPCThreadID,
		"incident_id": productHarnessGatewayCancelIPCIncident,
	}, &acknowledgement); err != nil {
		t.Fatal(err)
	}
	if !acknowledgement.Cancelled || acknowledgement.ThreadID != productHarnessGatewayCancelIPCThreadID ||
		acknowledgement.IncidentID != productHarnessGatewayCancelIPCIncident {
		t.Fatalf("cancellation acknowledgement = %#v", acknowledgement)
	}

	cancelled := <-cancelledResult
	if err := <-cancelledError; err != nil {
		t.Fatal(err)
	}
	assertProductHarnessGatewayCancelIPCAttempt(
		t, cancelled, productHarnessGatewayCancelIPCIncident, "cancelled",
	)
	if observable, cancelledRead := appServer.firstTurnResponseState(); !observable || !cancelledRead {
		t.Fatalf(
			"native turn ID was not recovered after the cancelled read: observable=%t cancelled_read=%t",
			observable, cancelledRead,
		)
	}
	assertProductHarnessGatewayCancelIPCNativeInterrupt(t, appServer.snapshot())
	assertProductHarnessGatewayCancelIPCEvent(t, events.snapshot())

	restarted, err := api.NewPersistentLocalProductChatAPI(
		storePath, time.Now, nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := restarted.ChatThread(context.Background(), productHarnessGatewayCancelIPCThreadID)
	if err != nil {
		t.Fatal(err)
	}
	assertProductHarnessGatewayCancelIPCAttempt(
		t, persisted, productHarnessGatewayCancelIPCIncident, "cancelled",
	)

	var continued api.LocalProductChatThread
	if err := client.Call(context.Background(), "chat_message", map[string]any{
		"thread_id":  productHarnessGatewayCancelIPCThreadID,
		"content":    "answer on the same Codex Session after the stop",
		"profile_id": productHarnessGatewayCancelIPCProfile,
		"model_id":   productHarnessGatewayCancelIPCModel,
	}, &continued); err != nil {
		t.Fatal(err)
	}
	if len(continued.Segments) != 1 || len(continued.Attempts) != 2 ||
		continued.Attempts[0].Status != "cancelled" || continued.Attempts[1].Status != "succeeded" ||
		continued.Segments[0].RouteTransitionReviewDigest != "" ||
		continued.Attempts[0].RouteTransitionReviewDigest != "" ||
		continued.Attempts[1].RouteTransitionReviewDigest != "" ||
		len(continued.Messages) != 4 || continued.Messages[3].Content != "Codex same-session reply" {
		t.Fatalf("same Codex Session did not continue: %#v", continued)
	}
	requests := appServer.snapshot()
	if sessions.starts() != 1 || countProductHarnessGatewayCancelIPCMethod(requests, "thread/start") != 1 ||
		countProductHarnessGatewayCancelIPCMethod(requests, "turn/start") != 2 ||
		countProductHarnessGatewayCancelIPCMethod(requests, "turn/interrupt") != 1 ||
		requests[len(requests)-1].Method != "turn/start" ||
		requests[len(requests)-1].ThreadID != "codex-thread-1" {
		t.Fatalf("Codex Session was not reused: starts=%d requests=%#v", sessions.starts(), requests)
	}
}

func prepareProductHarnessGatewayCodexAuthFixture(t *testing.T, home string) {
	t.Helper()
	codexHome := filepath.Join(home, ".codex")
	if err := os.MkdirAll(codexHome, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(codexHome, "auth.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertProductHarnessGatewayCancelIPCAttempt(
	t *testing.T,
	thread api.LocalProductChatThread,
	incidentID string,
	status string,
) {
	t.Helper()
	if len(thread.Segments) != 1 || thread.Segments[0].RouteTransitionReviewDigest != "" ||
		len(thread.Attempts) != 1 || thread.Attempts[0].IncidentID != incidentID ||
		thread.Attempts[0].RouteTransitionReviewDigest != "" ||
		thread.Attempts[0].Status != status || thread.Attempts[0].CompletedAt.IsZero() {
		t.Fatalf("initial Segment/Attempt authority: segments=%#v attempts=%#v", thread.Segments, thread.Attempts)
	}
}

func assertProductHarnessGatewayCancelIPCNativeInterrupt(
	t *testing.T,
	requests []productHarnessGatewayCancelIPCAppServerRequest,
) {
	t.Helper()
	if countProductHarnessGatewayCancelIPCMethod(requests, "turn/start") != 1 ||
		countProductHarnessGatewayCancelIPCMethod(requests, "turn/interrupt") != 1 {
		t.Fatalf("cancelled native turn requests = %#v", requests)
	}
	var started, interrupted productHarnessGatewayCancelIPCAppServerRequest
	for _, request := range requests {
		switch request.Method {
		case "turn/start":
			started = request
		case "turn/interrupt":
			interrupted = request
		}
	}
	if started.ThreadID != "codex-thread-1" || interrupted.ThreadID != started.ThreadID ||
		interrupted.TurnID != "codex-turn-1" {
		t.Fatalf("native interrupt did not target the selected turn: %#v", requests)
	}
}

func assertProductHarnessGatewayCancelIPCEvent(
	t *testing.T,
	events []harnessgateway.Event,
) {
	t.Helper()
	cancelled, completed := 0, 0
	for _, event := range events {
		if event.ConversationID != productHarnessGatewayCancelIPCThreadID ||
			event.IncidentID != productHarnessGatewayCancelIPCIncident {
			continue
		}
		if event.ProviderAccountID != "" || event.CredentialRevision != 0 ||
			event.GovernancePolicyDigest != "" || event.RouteTransitionReviewDigest != "" {
			t.Fatalf("initial Codex Segment event carried route review authority: %#v", event)
		}
		switch event.Type {
		case harnessgateway.EventResponseCancelled:
			cancelled++
		case harnessgateway.EventResponseCompleted:
			completed++
		}
	}
	if cancelled != 1 || completed != 0 {
		t.Fatalf("selected Incident Gateway events: cancelled=%d completed=%d events=%#v",
			cancelled, completed, events)
	}
}

type productHarnessGatewayCancelIPCRoute struct {
	productReadRoute
	chat *api.LocalProductChatAPI
}

func (route *productHarnessGatewayCancelIPCRoute) SendChatMessage(
	ctx context.Context,
	request api.LocalProductChatMessageRequest,
) (api.LocalProductChatThread, error) {
	return route.chat.SendMessage(ctx, request)
}

func (route *productHarnessGatewayCancelIPCRoute) ReadChatThread(
	ctx context.Context,
	threadID string,
) (api.LocalProductChatThread, error) {
	return route.chat.ChatThread(ctx, threadID)
}

type productHarnessGatewayCancelIPCBindingResolver struct {
	bindings map[string]api.LocalProductConversationExecutionBinding
}

func (resolver *productHarnessGatewayCancelIPCBindingResolver) ResolveConversationExecutionBinding(
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

func productHarnessGatewayCancelIPCCodexBinding() api.LocalProductConversationExecutionBinding {
	return api.LocalProductConversationExecutionBinding{
		SchemaVersion: 4, HarnessAdapter: "codex", ProviderID: "openai",
		ProviderAccountID: "", CredentialRevision: 0,
		ModelID: productHarnessGatewayCancelIPCModel,
	}
}

func productHarnessGatewayCancelIPCPeerBinding() api.LocalProductConversationExecutionBinding {
	return api.LocalProductConversationExecutionBinding{
		SchemaVersion: 4, HarnessAdapter: "loom-native", ProviderID: "deepseek",
		ProviderAccountID: "", CredentialRevision: 0, ModelID: "deepseek-chat",
		ProviderAccountPolicyVersion: 0, ProviderAccountPolicyRevision: 0,
		ProviderAccountPolicyDigest: "",
		TrustDomain:                 "", RetentionMode: "", DataRegion: "",
	}
}

type productHarnessGatewayCancelIPCPeerResponder struct {
	mu       sync.Mutex
	requests []api.LocalProductConversationRequest
}

func (responder *productHarnessGatewayCancelIPCPeerResponder) Respond(
	_ context.Context,
	request api.LocalProductConversationRequest,
) (api.LocalProductConversationResponse, error) {
	responder.mu.Lock()
	responder.requests = append(responder.requests, request)
	responder.mu.Unlock()
	return api.LocalProductConversationResponse{Content: "peer conversation reply", Tentative: true}, nil
}

func (responder *productHarnessGatewayCancelIPCPeerResponder) calls() int {
	responder.mu.Lock()
	defer responder.mu.Unlock()
	return len(responder.requests)
}

func (responder *productHarnessGatewayCancelIPCPeerResponder) snapshot() []api.LocalProductConversationRequest {
	responder.mu.Lock()
	defer responder.mu.Unlock()
	return append([]api.LocalProductConversationRequest(nil), responder.requests...)
}

type productHarnessGatewayCancelIPCEventSink struct {
	mu     sync.Mutex
	events []harnessgateway.Event
}

func (sink *productHarnessGatewayCancelIPCEventSink) Record(event harnessgateway.Event) {
	sink.mu.Lock()
	sink.events = append(sink.events, event)
	sink.mu.Unlock()
}

func (sink *productHarnessGatewayCancelIPCEventSink) snapshot() []harnessgateway.Event {
	sink.mu.Lock()
	defer sink.mu.Unlock()
	return append([]harnessgateway.Event(nil), sink.events...)
}

type productHarnessGatewayCancelIPCSessionRunner struct {
	mu      sync.Mutex
	count   int
	session harnessadapter.HarnessStreamSession
}

func (runner *productHarnessGatewayCancelIPCSessionRunner) StartSession(
	_ context.Context,
	_ harnessadapter.HarnessSessionRequest,
) (harnessadapter.HarnessStreamSession, error) {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	runner.count++
	return runner.session, nil
}

func (runner *productHarnessGatewayCancelIPCSessionRunner) starts() int {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	return runner.count
}

type productHarnessGatewayCancelIPCAppServerRequest struct {
	Method   string
	ThreadID string
	TurnID   string
}

type productHarnessGatewayCancelIPCAppServer struct {
	mu                                sync.Mutex
	lines                             chan []byte
	requests                          []productHarnessGatewayCancelIPCAppServerRequest
	turns                             int
	firstTurnAccepted                 chan struct{}
	firstTurnOnce                     sync.Once
	pendingFirstTurnResponse          []byte
	firstTurnResponseObservable       bool
	firstTurnReadCancelledPreResponse bool
	closed                            bool
	waited                            bool
	aborted                           bool
}

func newProductHarnessGatewayCancelIPCAppServer() *productHarnessGatewayCancelIPCAppServer {
	return &productHarnessGatewayCancelIPCAppServer{
		lines: make(chan []byte, 16), firstTurnAccepted: make(chan struct{}),
	}
}

func (server *productHarnessGatewayCancelIPCAppServer) WriteLine(
	_ context.Context,
	payload []byte,
) error {
	var request struct {
		ID     string `json:"id"`
		Method string `json:"method"`
		Params struct {
			Model    string `json:"model"`
			ThreadID string `json:"threadId"`
			TurnID   string `json:"turnId"`
		} `json:"params"`
	}
	if json.Unmarshal(payload, &request) != nil || request.ID == "" || request.Method == "" {
		return errors.New("invalid Codex app-server fixture request")
	}
	server.mu.Lock()
	if request.Method == "turn/start" {
		server.turns++
	}
	turn := server.turns
	server.requests = append(server.requests, productHarnessGatewayCancelIPCAppServerRequest{
		Method: request.Method, ThreadID: request.Params.ThreadID, TurnID: request.Params.TurnID,
	})
	server.mu.Unlock()

	switch request.Method {
	case "initialize":
		server.enqueue(map[string]any{
			"id": request.ID,
			"result": map[string]any{
				"userAgent": "codex_cli_rs/0.144.1", "codexHome": "/private/codex-home",
				"platformFamily": "unix", "platformOs": "macos",
			},
		})
	case "thread/start":
		server.enqueue(map[string]any{
			"id": request.ID, "result": map[string]any{
				"thread": map[string]any{"id": "codex-thread-1"}, "model": request.Params.Model,
			},
		})
	case "turn/start":
		turnID := fmt.Sprintf("codex-turn-%d", turn)
		started := map[string]any{
			"id":     request.ID,
			"result": map[string]any{"turn": map[string]any{"id": turnID, "status": "inProgress"}},
		}
		if turn == 1 {
			server.holdFirstTurnResponse(started)
			server.firstTurnOnce.Do(func() { close(server.firstTurnAccepted) })
			return nil
		}
		server.enqueue(started)
		server.enqueue(map[string]any{
			"method": "item/completed",
			"params": map[string]any{
				"threadId": request.Params.ThreadID, "turnId": turnID,
				"item": map[string]any{"type": "agentMessage", "text": "Codex same-session reply"},
			},
		})
		server.enqueue(map[string]any{
			"method": "thread/tokenUsage/updated",
			"params": map[string]any{
				"threadId": request.Params.ThreadID, "turnId": turnID,
				"tokenUsage": map[string]any{"last": map[string]any{
					"inputTokens": 2, "cachedInputTokens": 0, "outputTokens": 3,
					"reasoningOutputTokens": 0, "totalTokens": 5,
				}},
			},
		})
		server.enqueue(map[string]any{
			"method": "turn/completed",
			"params": map[string]any{
				"threadId": request.Params.ThreadID,
				"turn":     map[string]any{"id": turnID, "status": "completed"},
			},
		})
	case "turn/interrupt":
		server.enqueue(map[string]any{"id": request.ID, "result": map[string]any{}})
		server.enqueue(map[string]any{
			"method": "turn/completed",
			"params": map[string]any{
				"threadId": request.Params.ThreadID,
				"turn":     map[string]any{"id": request.Params.TurnID, "status": "interrupted"},
			},
		})
	default:
		return errors.New("unexpected Codex app-server fixture method")
	}
	return nil
}

func (server *productHarnessGatewayCancelIPCAppServer) enqueue(value any) {
	payload, _ := json.Marshal(value)
	server.lines <- payload
}

func (server *productHarnessGatewayCancelIPCAppServer) holdFirstTurnResponse(value any) {
	payload, _ := json.Marshal(value)
	server.mu.Lock()
	server.pendingFirstTurnResponse = payload
	server.mu.Unlock()
}

func (server *productHarnessGatewayCancelIPCAppServer) releaseFirstTurnResponseAfterCancellation() {
	server.mu.Lock()
	payload := server.pendingFirstTurnResponse
	server.pendingFirstTurnResponse = nil
	if len(payload) != 0 {
		server.firstTurnReadCancelledPreResponse = true
		server.firstTurnResponseObservable = true
	}
	server.mu.Unlock()
	if len(payload) != 0 {
		server.lines <- payload
	}
}

func (server *productHarnessGatewayCancelIPCAppServer) firstTurnResponseState() (
	observable bool,
	cancelledRead bool,
) {
	server.mu.Lock()
	defer server.mu.Unlock()
	return server.firstTurnResponseObservable, server.firstTurnReadCancelledPreResponse
}

func (server *productHarnessGatewayCancelIPCAppServer) ReadLine(
	ctx context.Context,
) ([]byte, error) {
	select {
	case line, ok := <-server.lines:
		if !ok {
			return nil, io.EOF
		}
		return line, nil
	case <-ctx.Done():
		server.releaseFirstTurnResponseAfterCancellation()
		return nil, ctx.Err()
	}
}

func (server *productHarnessGatewayCancelIPCAppServer) CloseInput() error {
	server.mu.Lock()
	defer server.mu.Unlock()
	if !server.closed {
		server.closed = true
		close(server.lines)
	}
	return nil
}

func (server *productHarnessGatewayCancelIPCAppServer) Wait(
	context.Context,
) (harnessadapter.HarnessCommandResult, error) {
	server.mu.Lock()
	server.waited = true
	server.mu.Unlock()
	return harnessadapter.HarnessCommandResult{ExitCode: 0}, nil
}

func (server *productHarnessGatewayCancelIPCAppServer) Abort() error {
	server.mu.Lock()
	server.aborted = true
	server.mu.Unlock()
	return nil
}

func (server *productHarnessGatewayCancelIPCAppServer) snapshot() []productHarnessGatewayCancelIPCAppServerRequest {
	server.mu.Lock()
	defer server.mu.Unlock()
	return append([]productHarnessGatewayCancelIPCAppServerRequest(nil), server.requests...)
}

func countProductHarnessGatewayCancelIPCMethod(
	requests []productHarnessGatewayCancelIPCAppServerRequest,
	method string,
) int {
	count := 0
	for _, request := range requests {
		if request.Method == method {
			count++
		}
	}
	return count
}
