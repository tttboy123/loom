package main

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/controltool"
	credentialvault "loom-pi-rebuild/internal/credentials/vault"
	"loom-pi-rebuild/internal/provider"
	"loom-pi-rebuild/internal/runtime/harnessadapter"
)

func TestProductOpenCodeSegmentBackendReturnsRegistryActionProposal(t *testing.T) {
	root := t.TempDir()
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	runner := &productOpenCodeSegmentRunnerFixture{callMissionTool: true}
	controlGateway := &productClaudeCodeControlGatewayFixture{}
	gateway := openProductOpenCodeSegmentGatewayForTest(
		t, root, runner, registry, controlGateway,
		&profileConversationLeaseRecorder{}, nil,
	)
	request := productOpenCodeSegmentRequestFixture(
		provider.OpenCodeConversationProfileID,
		provider.OpenCodeConversationDefaultModel,
		api.LocalProductConversationExecutionBinding{
			SchemaVersion: 4, HarnessAdapter: "opencode", ProviderID: "opencode",
			ModelID: provider.OpenCodeConversationDefaultModel,
		},
	)
	response, err := gateway.Respond(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if response.Content != "I prepared a governed Mission proposal." ||
		len(response.ActionProposals) != 1 ||
		response.ActionProposals[0].ToolID != controltool.ToolMissionsCreatePreview ||
		response.ActionProposals[0].AttemptID != request.AttemptID {
		t.Fatalf("OpenCode control response = %#v", response)
	}
	assertProductActionProposalFrozenTurn(
		t, response.ActionProposals[0], request, registry,
	)
	if runner.request.ControlMCP.URL == "" || runner.request.ControlMCP.Token == "" ||
		len(runner.secret) != 0 || controlGateway.readCalls != 1 ||
		controlGateway.proposalCalls != 1 {
		t.Fatalf("OpenCode control request = %#v secret=%d", runner.request, len(runner.secret))
	}
	if !strings.Contains(runner.request.SystemPrompt, "loom_missions_create_preview") ||
		!strings.Contains(runner.request.SystemPrompt, "prose-only") ||
		!strings.Contains(runner.request.SystemPrompt, "harness=opencode") {
		t.Fatalf("OpenCode system prompt did not freeze the control Registry: %q", runner.request.SystemPrompt)
	}
}

func TestProductOpenCodeSegmentBackendStopsAfterTerminalProposalAndKeepsIt(t *testing.T) {
	root := t.TempDir()
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	runner := &productOpenCodeSegmentRunnerFixture{
		callMissionTool: true, waitForCancellationAfterMission: true,
	}
	gateway := openProductOpenCodeSegmentGatewayForTest(
		t, root, runner, registry, &productClaudeCodeControlGatewayFixture{},
		&profileConversationLeaseRecorder{}, nil,
	)
	request := productOpenCodeSegmentRequestFixture(
		provider.OpenCodeConversationProfileID,
		provider.OpenCodeConversationDefaultModel,
		api.LocalProductConversationExecutionBinding{
			SchemaVersion: 4, HarnessAdapter: "opencode", ProviderID: "opencode",
			ModelID: provider.OpenCodeConversationDefaultModel,
		},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 750*time.Millisecond)
	defer cancel()
	response, err := gateway.Respond(ctx, request)
	if err != nil || response.Content != "I prepared a Loom proposal for your review." ||
		len(response.ActionProposals) != 1 ||
		response.ActionProposals[0].ToolID != controltool.ToolMissionsCreatePreview ||
		len(response.CompletedControlTools) != 2 || !runner.missionResponseReceived ||
		!runner.canceledAfterMission {
		t.Fatalf(
			"response=%#v received=%t canceled=%t error=%v",
			response, runner.missionResponseReceived, runner.canceledAfterMission, err,
		)
	}
	if ctx.Err() != nil {
		t.Fatalf("terminal Proposal waited for the caller deadline: %v", ctx.Err())
	}
}

func TestProductOpenCodeSegmentBackendDoesNotRecoverCancellationWithoutProposal(t *testing.T) {
	root := t.TempDir()
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	runner := &productOpenCodeSegmentRunnerFixture{waitForCancellationWithoutMission: true}
	gateway := openProductOpenCodeSegmentGatewayForTest(
		t, root, runner, registry, &productClaudeCodeControlGatewayFixture{},
		&profileConversationLeaseRecorder{}, nil,
	)
	request := productOpenCodeSegmentRequestFixture(
		provider.OpenCodeConversationProfileID,
		provider.OpenCodeConversationDefaultModel,
		api.LocalProductConversationExecutionBinding{
			SchemaVersion: 4, HarnessAdapter: "opencode", ProviderID: "opencode",
			ModelID: provider.OpenCodeConversationDefaultModel,
		},
	)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	response, err := gateway.Respond(ctx, request)
	if err == nil || !errors.Is(err, context.DeadlineExceeded) ||
		response.Content != "" || len(response.ActionProposals) != 0 {
		t.Fatalf("response=%#v error=%v", response, err)
	}
}

func TestProductOpenCodeSegmentBackendFreezesExactBrokeredCredential(t *testing.T) {
	root := t.TempDir()
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	runner := &productOpenCodeSegmentRunnerFixture{content: "brokered OpenCode reply"}
	leases := &profileConversationLeaseRecorder{}
	identity := credentialvault.CredentialIdentity{
		ProviderID: "deepseek", ProviderAccountID: "deepseek.team",
		CredentialReference: "credential-ref-deepseek-team", CredentialRevision: 7,
	}
	gateway := openProductOpenCodeSegmentGatewayForTest(
		t, root, runner, registry, &productClaudeCodeControlGatewayFixture{},
		leases, func(profileID string) (credentialvault.CredentialIdentity, bool) {
			return identity, profileID == "conversation-opencode-deepseek-team-r7"
		},
	)
	request := productOpenCodeSegmentRequestFixture(
		"conversation-opencode-deepseek-team-r7", "deepseek/deepseek-chat",
		api.LocalProductConversationExecutionBinding{
			SchemaVersion: 4, HarnessAdapter: "opencode", ProviderID: "deepseek",
			ProviderAccountID: "deepseek.team", CredentialRevision: 7,
			ModelID: "deepseek/deepseek-chat",
		},
	)
	response, err := gateway.Respond(context.Background(), request)
	if err != nil || response.Content != "brokered OpenCode reply" {
		t.Fatalf("response=%#v error=%v", response, err)
	}
	if leases.calls != 1 || leases.identity != identity ||
		string(runner.secret) != "account-private-key" || !runner.request.RequiresCredential {
		t.Fatalf("lease=%#v calls=%d request=%#v secret=%q", leases.identity, leases.calls, runner.request, runner.secret)
	}
}

func TestProductOpenCodeSegmentFailurePreservesStructuredProviderFailure(t *testing.T) {
	_, providerErr := provider.DecodeOpenCodeText([]byte(
		`{"type":"error","error":{"name":"APIError","data":{"statusCode":503,"isRetryable":true,"message":"must not escape","responseBody":"secret body"}}}`,
	), 4096)
	if providerErr == nil {
		t.Fatal("OpenCode error event was accepted")
	}

	failure := productOpenCodeSegmentFailure(providerErr)
	info, ok := api.LocalProductConversationDispatchFailureDetails(failure)
	if !ok || info.Code != "provider_unavailable" || info.Stage != "provider_http" ||
		info.HTTPStatus != 503 || !info.Retryable || info.UserMessage == "" ||
		strings.Contains(failure.Error(), "must not escape") ||
		strings.Contains(failure.Error(), "secret body") {
		t.Fatalf("failure=%v info=%#v ok=%t", failure, info, ok)
	}
}

func TestProductOpenCodeSegmentFailurePublishesInternalTurnTimeout(t *testing.T) {
	failure := productOpenCodeSegmentFailure(harnessadapter.ErrHarnessProviderTimeout)
	info, ok := api.LocalProductConversationDispatchFailureDetails(failure)
	if !ok || info.Code != "timeout" || info.Stage != "provider_connect" ||
		!info.Retryable || info.ProviderCode != "opencode_turn_timeout" ||
		info.UserMessage == "" || !errors.Is(failure, api.ErrLocalProductChatUnavailable) {
		t.Fatalf("failure=%v info=%#v ok=%t", failure, info, ok)
	}
}

func openProductOpenCodeSegmentGatewayForTest(
	t *testing.T,
	root string,
	runner harnessadapter.HarnessProcessRunner,
	registry *controltool.Registry,
	controlGateway controltool.Gateway,
	leases productCredentialLeaseAccess,
	resolve productOpenCodeCredentialResolver,
) *productHarnessGatewayConversationResponder {
	t.Helper()
	if resolve == nil {
		resolve = func(string) (credentialvault.CredentialIdentity, bool) {
			return credentialvault.CredentialIdentity{}, false
		}
	}
	workspace := filepath.Join(root, "workspace")
	privateRoot := filepath.Join(root, "private")
	for _, directory := range []string{workspace, privateRoot} {
		if err := prepareProductHarnessGatewayWorkspace(directory); err != nil {
			t.Fatal(err)
		}
	}
	gateway, err := newProductHarnessGatewayConversationResponder(
		productHarnessGatewayConversationConfig{
			Executor: &productHarnessConversationResponderFixture{}, WorkspacePath: workspace,
			Events: &productHarnessGatewayEventFixture{},
			Now:    func() time.Time { return time.Unix(9, 0).UTC() },
			OpenCodeSegment: &productOpenCodeSegmentBackendConfig{
				ExecutablePath: "/opt/loom/bin/opencode", HomePath: "/Users/test",
				PrivateRoot: privateRoot, Timeout: time.Minute, MaxOutputBytes: 4096,
				Runner: runner, CredentialLeases: leases, ResolveCredential: resolve,
				ControlRegistry: registry, ControlGateway: controlGateway,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gateway.Close(context.Background()) })
	return gateway
}

func productOpenCodeSegmentRequestFixture(
	profileID string,
	modelID string,
	binding api.LocalProductConversationExecutionBinding,
) api.LocalProductConversationRequest {
	return api.LocalProductConversationRequest{
		ThreadID: "conversation-opencode", SegmentID: "segment-opencode",
		AttemptID: "attempt-opencode-1", IncidentID: "incident-opencode-1",
		ProfileID: profileID, ModelID: modelID, ExecutionBinding: &binding,
		ContextPrompt:               "capsule prompt",
		ContextCapsuleDigest:        productHarnessGatewayDigest("capsule:opencode"),
		SegmentContextCapsuleDigest: productHarnessGatewayDigest("capsule:opencode"),
		SegmentBindingDigest:        productHarnessGatewayDigest("binding:opencode"),
		BindingDigest:               productHarnessGatewayDigest("attempt:opencode"),
		CatalogDigest:               controltool.DigestBytes([]byte("catalog")),
		Messages: []api.LocalProductChatMessage{{
			MessageID: "message-opencode-1", SegmentID: "segment-opencode",
			Role: "user", Content: "Create a Mission to ship the release",
		}},
	}
}

type productOpenCodeSegmentRunnerFixture struct {
	callMissionTool                   bool
	waitForCancellationAfterMission   bool
	waitForCancellationWithoutMission bool
	missionResponseReceived           bool
	canceledAfterMission              bool
	content                           string
	request                           harnessadapter.HarnessProcessRequest
	secret                            []byte
}

func (runner *productOpenCodeSegmentRunnerFixture) RunHarness(
	ctx context.Context,
	request harnessadapter.HarnessProcessRequest,
	secret []byte,
) (harnessadapter.HarnessProcessResult, error) {
	runner.request = request
	runner.secret = append([]byte(nil), secret...)
	defer func() {
		for index := range request.Prompt {
			request.Prompt[index] = 0
		}
	}()
	if runner.callMissionTool {
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
			return harnessadapter.HarnessProcessResult{}, err
		}
		runner.missionResponseReceived = true
		terminal, acknowledgeErr := harnessadapter.AcknowledgeHarnessControlToolCompletion(
			ctx, request.ControlMCP,
		)
		if acknowledgeErr != nil && ctx.Err() == nil {
			return harnessadapter.HarnessProcessResult{}, acknowledgeErr
		}
		if acknowledgeErr == nil && !terminal {
			return harnessadapter.HarnessProcessResult{}, harnessadapter.ErrHarnessControlMCP
		}
		if runner.waitForCancellationAfterMission {
			<-ctx.Done()
			runner.canceledAfterMission = true
			return harnessadapter.HarnessProcessResult{}, ctx.Err()
		}
		return harnessadapter.HarnessProcessResult{
			Content: "I prepared a governed Mission proposal.",
		}, nil
	}
	if runner.waitForCancellationWithoutMission {
		<-ctx.Done()
		return harnessadapter.HarnessProcessResult{}, ctx.Err()
	}
	return harnessadapter.HarnessProcessResult{Content: runner.content}, nil
}
