package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/controltool"
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

func TestProductClaudeCodeSegmentBackendReturnsRegistryActionProposal(t *testing.T) {
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
	runner := &productClaudeCodeControlRunnerFixture{}
	responder, err := newProductClaudeCodeConversationResponder(
		productClaudeCodeConversationConfig{
			ExecutablePath: executablePath, HomePath: filepath.Join(root, "home"),
			WorkspacePath: filepath.Join(root, "workspace"),
			PrivateRoot:   filepath.Join(root, "private"), Runner: runner,
			Timeout: time.Minute, MaxOutputBytes: 4096,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	controlGateway := &productClaudeCodeControlGatewayFixture{}
	gateway, err := newProductHarnessGatewayConversationResponder(
		productHarnessGatewayConversationConfig{
			Executor:      &productHarnessConversationResponderFixture{},
			WorkspacePath: filepath.Join(root, "workspace"),
			Events:        &productHarnessGatewayEventFixture{},
			Now:           func() time.Time { return time.Unix(9, 0).UTC() },
			ClaudeCodeSegment: &productClaudeCodeSegmentBackendConfig{
				Responder: responder, ControlRegistry: registry, ControlGateway: controlGateway,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gateway.Close(context.Background()) })

	request := productClaudeCodeSegmentRequestFixture()
	request.AttemptID = "attempt-action-1"
	request.IncidentID = "incident-action-1"
	request.BindingDigest = productHarnessGatewayDigest(request.AttemptID)
	request.CatalogDigest = controltool.DigestBytes([]byte("catalog"))
	request.Messages = []api.LocalProductChatMessage{{
		MessageID: "message-action-1", SegmentID: request.SegmentID,
		Role: "user", Content: "Create a Mission to ship the release",
	}}
	response, err := gateway.Respond(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if response.Content != "I prepared a governed Mission proposal." ||
		len(response.ActionProposals) != 1 ||
		response.ActionProposals[0].ToolID != controltool.ToolMissionsCreatePreview ||
		response.ActionProposals[0].Argument != "Ship the governed release" ||
		response.ActionProposals[0].AttemptID != request.AttemptID {
		t.Fatalf("Claude control response = %#v", response)
	}
	assertProductActionProposalFrozenTurn(
		t, response.ActionProposals[0], request, registry,
	)
	if controlGateway.calls != 2 || controlGateway.readCalls != 1 ||
		controlGateway.proposalCalls != 1 || runner.controlURL == "" ||
		runner.controlToken == "" ||
		!strings.Contains(runner.systemPrompt, "loom_missions_create_preview") ||
		!strings.Contains(runner.systemPrompt, "harness=claude-code") {
		t.Fatalf(
			"control calls=%d read=%d proposal=%d URL=%q token=%t prompt=%q",
			controlGateway.calls, controlGateway.readCalls,
			controlGateway.proposalCalls, runner.controlURL,
			runner.controlToken != "", runner.systemPrompt,
		)
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

type productClaudeCodeControlRunnerFixture struct {
	controlURL   string
	controlToken string
	systemPrompt string
}

func (runner *productClaudeCodeControlRunnerFixture) RunHarness(
	ctx context.Context,
	request harnessadapter.HarnessProcessRequest,
	secret []byte,
) (harnessadapter.HarnessProcessResult, error) {
	defer func() {
		for index := range request.Prompt {
			request.Prompt[index] = 0
		}
	}()
	if len(secret) != 0 || request.ControlMCP.URL == "" ||
		request.ControlMCP.Token == "" {
		return harnessadapter.HarnessProcessResult{}, harnessadapter.ErrHarnessProtocol
	}
	runner.controlURL = request.ControlMCP.URL
	runner.controlToken = request.ControlMCP.Token
	runner.systemPrompt = request.SystemPrompt
	if err := callProductHarnessControlMCP(
		ctx, request.ControlMCP.URL, request.ControlMCP.Token,
		"loom_workspace_status", json.RawMessage(`{}`),
		[]byte(`"workspace_id":"workspace-test"`),
	); err != nil {
		return harnessadapter.HarnessProcessResult{}, err
	}
	if err := callProductHarnessControlMCP(
		ctx, request.ControlMCP.URL, request.ControlMCP.Token,
		"loom_missions_create_preview",
		json.RawMessage(`{"objective":"Ship the governed release"}`),
		[]byte(`"requires_confirmation":true`),
	); err != nil {
		return harnessadapter.HarnessProcessResult{}, harnessadapter.ErrHarnessProtocol
	}
	return harnessadapter.HarnessProcessResult{
		Content: "I prepared a governed Mission proposal.",
	}, nil
}

type productClaudeCodeControlGatewayFixture struct {
	calls         int
	readCalls     int
	proposalCalls int
}

func (gateway *productClaudeCodeControlGatewayFixture) Call(
	_ context.Context,
	turn controltool.TurnContext,
	call controltool.Call,
) (controltool.Result, error) {
	gateway.calls++
	if call.ToolID == controltool.ToolWorkspaceStatus {
		if string(call.Arguments) != `{}` || !turn.Workspace.Valid() {
			return controltool.Result{}, controltool.ErrInvalidCall
		}
		gateway.readCalls++
		return controltool.Result{Content: json.RawMessage(
			`{"schema_version":1,"workspace":{"workspace_id":"workspace-test","workspace_digest":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}}`,
		)}, nil
	}
	if call.ToolID != controltool.ToolMissionsCreatePreview {
		return controltool.Result{}, controltool.ErrInvalidCall
	}
	gateway.proposalCalls++
	var arguments struct {
		Objective string `json:"objective"`
	}
	if json.Unmarshal(call.Arguments, &arguments) != nil ||
		arguments.Objective != "Ship the governed release" {
		return controltool.Result{}, controltool.ErrInvalidCall
	}
	createdAt := time.Date(2026, 8, 28, 12, 0, 0, 0, time.UTC)
	proposal, err := controltool.NewConversationActionProposal(
		controltool.ConversationActionProposalInput{
			ProposalID: "proposal-claude-action-1", ToolID: call.ToolID,
			TargetConversationID: turn.ConversationID,
			TargetContentDigest:  controltool.DigestBytes([]byte("target")),
			Argument:             arguments.Objective, SegmentID: turn.SegmentID,
			Route: &turn.Route, Workspace: &turn.Workspace,
			RegistryDigest: turn.RegistryDigest, IncidentID: turn.IncidentID,
			AttemptID: turn.AttemptID, CreatedAt: createdAt,
			ExpiresAt: createdAt.Add(5 * time.Minute),
		},
	)
	if err != nil {
		return controltool.Result{}, err
	}
	return controltool.Result{
		Content:        json.RawMessage(`{"requires_confirmation":true}`),
		ActionProposal: &proposal,
	}, nil
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
