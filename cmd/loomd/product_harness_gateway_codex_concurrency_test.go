package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/harnessgateway"
	"loom-pi-rebuild/internal/runtime/harnessadapter"
)

func TestProductHarnessGatewayDistinctCodexConversationsOverlapRealSegmentBackends(t *testing.T) {
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	privateRoot := filepath.Join(root, "codex-private")
	home := filepath.Join(root, "home")
	for _, directory := range []string{workspace, privateRoot, home} {
		if err := os.MkdirAll(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	prepareProductHarnessGatewayCodexAuthFixture(t, home)

	barrier := newProductHarnessGatewayCodexConcurrencyBarrier()
	sessions := &productHarnessGatewayCodexConcurrencyRunner{barrier: barrier}
	events := &productHarnessGatewayCancelIPCEventSink{}
	direct := &productHarnessGatewayCancelIPCPeerResponder{}
	responder, err := newProductHarnessGatewayConversationResponder(
		productHarnessGatewayConversationConfig{
			Executor: direct, WorkspacePath: workspace, Events: events,
			Now: func() time.Time { return time.Unix(1_800_000_300, 0).UTC() },
			CodexSegment: &productCodexSegmentBackendConfig{
				ExecutablePath: "/opt/loom/bin/codex", HomePath: home,
				PrivateRoot: privateRoot, Sessions: sessions,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	closed := false
	t.Cleanup(func() {
		barrier.releaseTurns()
		if !closed {
			if closeErr := responder.Close(context.Background()); closeErr != nil {
				t.Errorf("close Harness Gateway responder: %v", closeErr)
			}
		}
	})

	chat, err := api.NewPersistentLocalProductChatAPI(
		filepath.Join(root, "chat.json"), time.Now, responder,
	)
	if err != nil {
		t.Fatal(err)
	}
	resolver := &productHarnessGatewayCancelIPCBindingResolver{bindings: map[string]api.LocalProductConversationExecutionBinding{
		productHarnessGatewayCancelIPCProfile: productHarnessGatewayCancelIPCCodexBinding(),
	}}
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
				ThreadID:    fmt.Sprintf("conversation-codex-overlap-%d", index),
				Content:     fmt.Sprintf("hold Codex overlap turn %d", index),
				ProfileID:   productHarnessGatewayCancelIPCProfile,
				ModelID:     productHarnessGatewayCancelIPCModel,
				ContextMode: api.ContextModeStartClean,
				IncidentID:  fmt.Sprintf("incident-codex-overlap-%d", index),
			})
			results <- result{thread: thread, err: sendErr}
		}()
	}

	select {
	case <-barrier.bothStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("distinct Codex Conversations did not overlap before either turn terminated")
	}

	streams, requests := sessions.snapshot()
	if len(streams) != 2 || len(requests) != 2 || streams[0] == streams[1] {
		t.Fatalf("native Codex Segment sessions: streams=%d requests=%d", len(streams), len(requests))
	}
	if requests[0].Directory != workspace || requests[1].Directory != workspace ||
		strings.Join(requests[0].Arguments, "\n") == strings.Join(requests[1].Arguments, "\n") {
		t.Fatalf("Codex Segment process authority was not distinct: %#v", requests)
	}
	nativeThreads := map[string]bool{}
	for _, stream := range streams {
		state := stream.snapshot()
		if state.turnStarts != 1 || state.turnCompleted || state.closed || state.waited || state.aborted {
			t.Fatalf("Codex turn terminated before overlap barrier release: %#v", state)
		}
		for _, request := range state.requests {
			if request.Method == "turn/start" {
				nativeThreads[request.ThreadID] = true
			}
		}
	}
	if len(nativeThreads) != 2 || nativeThreads[""] {
		t.Fatalf("distinct native Codex threads = %#v", nativeThreads)
	}
	openingSessions := map[string]string{}
	for _, event := range events.snapshot() {
		if event.Type == harnessgateway.EventSessionOpening {
			if event.HarnessID != harnessgateway.HarnessCodex ||
				event.BackendID != productCodexSegmentBackendID ||
				event.RouteTransitionReviewDigest != "" {
				t.Fatalf("production Codex Gateway route = %#v", event)
			}
			openingSessions[event.ConversationID] = event.SessionID
		}
	}
	if len(openingSessions) != 2 ||
		openingSessions["conversation-codex-overlap-1"] == "" ||
		openingSessions["conversation-codex-overlap-2"] == "" ||
		openingSessions["conversation-codex-overlap-1"] == openingSessions["conversation-codex-overlap-2"] {
		t.Fatalf("distinct Gateway Session authority = %#v", openingSessions)
	}
	if direct.calls() != 0 {
		t.Fatalf("Codex Conversations escaped to compatibility executor: calls=%d", direct.calls())
	}

	barrier.releaseTurns()
	seen := map[string]bool{}
	for range 2 {
		select {
		case got := <-results:
			if got.err != nil {
				t.Fatal(got.err)
			}
			if len(got.thread.Segments) != 1 || len(got.thread.Attempts) != 1 ||
				got.thread.Segments[0].RouteTransitionReviewDigest != "" ||
				got.thread.Attempts[0].RouteTransitionReviewDigest != "" ||
				got.thread.Attempts[0].Status != "succeeded" || len(got.thread.Messages) != 2 {
				t.Fatalf("overlapping Codex Conversation = %#v", got.thread)
			}
			seen[got.thread.ThreadID] = true
		case <-time.After(5 * time.Second):
			t.Fatal("overlapping Codex Conversation did not finish after release")
		}
	}
	if !seen["conversation-codex-overlap-1"] || !seen["conversation-codex-overlap-2"] {
		t.Fatalf("completed Codex Conversations = %#v", seen)
	}

	if err := responder.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	closed = true
	for _, stream := range streams {
		state := stream.snapshot()
		if !state.turnCompleted || !state.closed || !state.waited || state.aborted {
			t.Fatalf("Codex Segment Session cleanup = %#v", state)
		}
	}
}

type productHarnessGatewayCodexConcurrencyBarrier struct {
	mu          sync.Mutex
	started     int
	bothStarted chan struct{}
	release     chan struct{}
	startedOnce sync.Once
	releaseOnce sync.Once
}

func newProductHarnessGatewayCodexConcurrencyBarrier() *productHarnessGatewayCodexConcurrencyBarrier {
	return &productHarnessGatewayCodexConcurrencyBarrier{
		bothStarted: make(chan struct{}), release: make(chan struct{}),
	}
}

func (barrier *productHarnessGatewayCodexConcurrencyBarrier) acceptTurn() {
	barrier.mu.Lock()
	barrier.started++
	if barrier.started == 2 {
		barrier.startedOnce.Do(func() { close(barrier.bothStarted) })
	}
	barrier.mu.Unlock()
}

func (barrier *productHarnessGatewayCodexConcurrencyBarrier) releaseTurns() {
	barrier.releaseOnce.Do(func() { close(barrier.release) })
}

type productHarnessGatewayCodexConcurrencyRunner struct {
	mu       sync.Mutex
	barrier  *productHarnessGatewayCodexConcurrencyBarrier
	streams  []*productHarnessGatewayCodexConcurrencyAppServer
	requests []harnessadapter.HarnessSessionRequest
}

func (runner *productHarnessGatewayCodexConcurrencyRunner) StartSession(
	ctx context.Context,
	request harnessadapter.HarnessSessionRequest,
) (harnessadapter.HarnessStreamSession, error) {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	index := len(runner.streams) + 1
	stream := newProductHarnessGatewayCodexConcurrencyAppServer(
		index, ctx, runner.barrier,
	)
	runner.streams = append(runner.streams, stream)
	runner.requests = append(runner.requests, request)
	return stream, nil
}

func (runner *productHarnessGatewayCodexConcurrencyRunner) snapshot() (
	[]*productHarnessGatewayCodexConcurrencyAppServer,
	[]harnessadapter.HarnessSessionRequest,
) {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	return append([]*productHarnessGatewayCodexConcurrencyAppServer(nil), runner.streams...),
		append([]harnessadapter.HarnessSessionRequest(nil), runner.requests...)
}

type productHarnessGatewayCodexConcurrencyRequest struct {
	Method   string
	ThreadID string
}

type productHarnessGatewayCodexConcurrencyState struct {
	requests      []productHarnessGatewayCodexConcurrencyRequest
	turnStarts    int
	turnCompleted bool
	closed        bool
	waited        bool
	aborted       bool
}

type productHarnessGatewayCodexConcurrencyAppServer struct {
	mu            sync.Mutex
	index         int
	lifetime      context.Context
	barrier       *productHarnessGatewayCodexConcurrencyBarrier
	lines         chan []byte
	requests      []productHarnessGatewayCodexConcurrencyRequest
	turnStarts    int
	turnCompleted bool
	closed        bool
	waited        bool
	aborted       bool
}

func newProductHarnessGatewayCodexConcurrencyAppServer(
	index int,
	lifetime context.Context,
	barrier *productHarnessGatewayCodexConcurrencyBarrier,
) *productHarnessGatewayCodexConcurrencyAppServer {
	return &productHarnessGatewayCodexConcurrencyAppServer{
		index: index, lifetime: lifetime, barrier: barrier, lines: make(chan []byte, 16),
	}
}

func (server *productHarnessGatewayCodexConcurrencyAppServer) WriteLine(
	_ context.Context,
	payload []byte,
) error {
	var request struct {
		ID     string `json:"id"`
		Method string `json:"method"`
		Params struct {
			Model    string `json:"model"`
			ThreadID string `json:"threadId"`
		} `json:"params"`
	}
	if json.Unmarshal(payload, &request) != nil || request.ID == "" || request.Method == "" {
		return errors.New("invalid Codex concurrency fixture request")
	}
	server.mu.Lock()
	server.requests = append(server.requests, productHarnessGatewayCodexConcurrencyRequest{
		Method: request.Method, ThreadID: request.Params.ThreadID,
	})
	if request.Method == "turn/start" {
		server.turnStarts++
	}
	server.mu.Unlock()

	threadID := fmt.Sprintf("codex-overlap-thread-%d", server.index)
	turnID := fmt.Sprintf("codex-overlap-turn-%d", server.index)
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
				"thread": map[string]any{"id": threadID}, "model": request.Params.Model,
			},
		})
	case "turn/start":
		if request.Params.ThreadID != threadID {
			return errors.New("turn/start targeted the wrong Codex thread")
		}
		server.enqueue(map[string]any{
			"id":     request.ID,
			"result": map[string]any{"turn": map[string]any{"id": turnID, "status": "inProgress"}},
		})
		server.barrier.acceptTurn()
		go server.completeTurnAfterRelease(threadID, turnID)
	default:
		return errors.New("unexpected Codex concurrency fixture method")
	}
	return nil
}

func (server *productHarnessGatewayCodexConcurrencyAppServer) completeTurnAfterRelease(
	threadID string,
	turnID string,
) {
	select {
	case <-server.barrier.release:
	case <-server.lifetime.Done():
		return
	}
	server.enqueue(map[string]any{
		"method": "item/completed",
		"params": map[string]any{
			"threadId": threadID, "turnId": turnID,
			"item": map[string]any{
				"type": "agentMessage", "text": fmt.Sprintf("Codex overlap reply %d", server.index),
			},
		},
	})
	server.enqueue(map[string]any{
		"method": "thread/tokenUsage/updated",
		"params": map[string]any{
			"threadId": threadID, "turnId": turnID,
			"tokenUsage": map[string]any{"last": map[string]any{
				"inputTokens": 3, "cachedInputTokens": 0, "outputTokens": 4,
				"reasoningOutputTokens": 0, "totalTokens": 7,
			}},
		},
	})
	server.enqueue(map[string]any{
		"method": "turn/completed",
		"params": map[string]any{
			"threadId": threadID,
			"turn":     map[string]any{"id": turnID, "status": "completed"},
		},
	})
	server.mu.Lock()
	server.turnCompleted = true
	server.mu.Unlock()
}

func (server *productHarnessGatewayCodexConcurrencyAppServer) enqueue(value any) {
	payload, _ := json.Marshal(value)
	server.lines <- payload
}

func (server *productHarnessGatewayCodexConcurrencyAppServer) ReadLine(
	ctx context.Context,
) ([]byte, error) {
	select {
	case line, ok := <-server.lines:
		if !ok {
			return nil, io.EOF
		}
		return line, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (server *productHarnessGatewayCodexConcurrencyAppServer) CloseInput() error {
	server.mu.Lock()
	defer server.mu.Unlock()
	if !server.closed {
		server.closed = true
		close(server.lines)
	}
	return nil
}

func (server *productHarnessGatewayCodexConcurrencyAppServer) Wait(
	context.Context,
) (harnessadapter.HarnessCommandResult, error) {
	server.mu.Lock()
	server.waited = true
	server.mu.Unlock()
	return harnessadapter.HarnessCommandResult{ExitCode: 0}, nil
}

func (server *productHarnessGatewayCodexConcurrencyAppServer) Abort() error {
	server.mu.Lock()
	server.aborted = true
	server.mu.Unlock()
	return nil
}

func (server *productHarnessGatewayCodexConcurrencyAppServer) snapshot() productHarnessGatewayCodexConcurrencyState {
	server.mu.Lock()
	defer server.mu.Unlock()
	return productHarnessGatewayCodexConcurrencyState{
		requests:   append([]productHarnessGatewayCodexConcurrencyRequest(nil), server.requests...),
		turnStarts: server.turnStarts, turnCompleted: server.turnCompleted,
		closed: server.closed, waited: server.waited, aborted: server.aborted,
	}
}
