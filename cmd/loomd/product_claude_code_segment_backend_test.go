package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/harnessgateway"
	"loom-pi-rebuild/internal/provider"
	"loom-pi-rebuild/internal/runtime/harnessadapter"
)

func TestProductClaudeCodeSegmentBackendReusesFrozenNativeSession(t *testing.T) {
	root := t.TempDir()
	executablePath := filepath.Join(root, "claude")
	if err := os.WriteFile(executablePath, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(root, "home"), filepath.Join(root, "workspace"),
		filepath.Join(root, "private"),
	} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	runner := &productClaudeCodeSegmentRunnerFixture{}
	claudeResponder, err := newProductClaudeCodeConversationResponder(
		productClaudeCodeConversationConfig{
			ExecutablePath: executablePath,
			HomePath:       filepath.Join(root, "home"),
			WorkspacePath:  filepath.Join(root, "workspace"),
			PrivateRoot:    filepath.Join(root, "private"),
			Runner:         runner,
			Timeout:        time.Minute,
			MaxOutputBytes: 4096,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	direct := &productHarnessConversationResponderFixture{}
	events := &productHarnessGatewayEventFixture{}
	gateway, err := newProductHarnessGatewayConversationResponder(
		productHarnessGatewayConversationConfig{
			Executor: direct, WorkspacePath: filepath.Join(root, "workspace"),
			Events: events, Now: func() time.Time { return time.Unix(9, 0).UTC() },
			ClaudeCodeSegment: &productClaudeCodeSegmentBackendConfig{
				Responder: claudeResponder,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := gateway.Close(context.Background()); closeErr != nil {
			t.Errorf("close Gateway: %v", closeErr)
		}
	})
	if gateway.backendIDs[harnessgateway.HarnessClaudeCode] != productClaudeCodeSegmentBackendID ||
		gateway.backendVersions[harnessgateway.HarnessClaudeCode] != productClaudeCodeSegmentBackendVersion {
		t.Fatalf("Claude Backend ids=%#v versions=%#v", gateway.backendIDs, gateway.backendVersions)
	}

	base := productClaudeCodeSegmentRequestFixture()
	for index, content := range []string{"first native turn", "second native turn"} {
		request := base
		request.AttemptID = "attempt-" + string(rune('1'+index))
		request.IncidentID = "incident-" + request.AttemptID
		request.BindingDigest = productHarnessGatewayDigest(request.AttemptID)
		request.Messages = []api.LocalProductChatMessage{{
			MessageID: "message-" + string(rune('1'+index)),
			SegmentID: base.SegmentID, Role: "user", Content: content,
		}}
		response, respondErr := gateway.Respond(context.Background(), request)
		if respondErr != nil || response.Content != "claude-segment-reply-"+string(rune('1'+index)) {
			t.Fatalf("response=%#v error=%v", response, respondErr)
		}
	}
	requests := runner.snapshot()
	if len(requests) != 2 || requests[0].NativeSessionID == "" ||
		requests[0].NativeSessionID != requests[1].NativeSessionID ||
		requests[0].ResumeNativeSession || !requests[1].ResumeNativeSession ||
		!strings.Contains(requests[0].prompt, "capsule prompt") ||
		!strings.Contains(requests[0].prompt, "first native turn") ||
		requests[1].prompt != "second native turn" {
		t.Fatalf("Claude native Session requests = %#v", requests)
	}
	if direct.calls() != 0 || events.count(harnessgateway.EventSessionOpening) != 1 ||
		events.count(harnessgateway.EventResponseCompleted) != 2 {
		t.Fatalf("direct=%d events=%#v", direct.calls(), events.snapshot())
	}
}

func productClaudeCodeSegmentRequestFixture() api.LocalProductConversationRequest {
	binding := &api.LocalProductConversationExecutionBinding{
		SchemaVersion: 4, HarnessAdapter: "claude-code", ProviderID: "anthropic",
		ModelID: provider.AnthropicConversationModelID,
	}
	return api.LocalProductConversationRequest{
		ThreadID: "conversation-claude", SegmentID: "segment-claude",
		ProfileID:                   provider.ClaudeCodeConversationProfileID,
		ModelID:                     provider.AnthropicConversationModelID,
		ContextPrompt:               "capsule prompt",
		ContextCapsuleDigest:        productHarnessGatewayDigest("capsule:claude-code"),
		SegmentContextCapsuleDigest: productHarnessGatewayDigest("capsule:claude-code"),
		ExecutionBinding:            binding,
		SegmentBindingDigest:        productHarnessGatewayDigest("binding:claude-code"),
	}
}

type productClaudeCodeSegmentRunnerRequest struct {
	NativeSessionID     string
	ResumeNativeSession bool
	prompt              string
}

type productClaudeCodeSegmentRunnerFixture struct {
	mu       sync.Mutex
	requests []productClaudeCodeSegmentRunnerRequest
}

func (runner *productClaudeCodeSegmentRunnerFixture) RunHarness(
	_ context.Context,
	request harnessadapter.HarnessProcessRequest,
	secret []byte,
) (harnessadapter.HarnessProcessResult, error) {
	if len(secret) != 0 {
		return harnessadapter.HarnessProcessResult{}, harnessadapter.ErrInvalidClaudeCodeAdapter
	}
	runner.mu.Lock()
	runner.requests = append(runner.requests, productClaudeCodeSegmentRunnerRequest{
		NativeSessionID:     request.NativeSessionID,
		ResumeNativeSession: request.ResumeNativeSession,
		prompt:              string(request.Prompt),
	})
	count := len(runner.requests)
	runner.mu.Unlock()
	for index := range request.Prompt {
		request.Prompt[index] = 0
	}
	return harnessadapter.HarnessProcessResult{
		Content: "claude-segment-reply-" + string(rune('0'+count)),
	}, nil
}

func (runner *productClaudeCodeSegmentRunnerFixture) snapshot() []productClaudeCodeSegmentRunnerRequest {
	runner.mu.Lock()
	defer runner.mu.Unlock()
	return append([]productClaudeCodeSegmentRunnerRequest(nil), runner.requests...)
}
