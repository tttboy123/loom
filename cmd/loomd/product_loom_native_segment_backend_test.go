package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/controltool"
	credentialvault "loom-pi-rebuild/internal/credentials/vault"
	"loom-pi-rebuild/internal/harnessgateway"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/provider"
)

func TestProductLoomNativeSegmentFreezesCredentialAndReturnsRegistryProposal(t *testing.T) {
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	credential := projection.ProviderCredentialRecord{
		ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
		CredentialReference: "credential-ref-deepseek-primary",
		Revision:            3, Status: "verified", AuthMode: "brokered",
	}
	leases := &profileConversationLeaseRecorder{}
	providerClient := &productLoomNativeControlProviderFixture{}
	router, err := newProductConversationProfileRouter(
		nil,
		func(providerID string) []projection.ProviderCredentialRecord {
			if providerID == credential.ProviderID {
				return []projection.ProviderCredentialRecord{credential}
			}
			return nil
		},
		leases,
		providerClient,
	)
	if err != nil {
		t.Fatal(err)
	}
	router.requireExecutionBinding = true
	profileID := provider.DeepSeekConversationAccountProfileID(
		credential.ProviderAccountID, credential.Revision,
	)
	binding, err := router.ResolveConversationExecutionBinding(
		context.Background(), profileID, provider.DeepSeekConversationModelID,
	)
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	workspace := filepath.Join(root, "workspace")
	if err := prepareProductHarnessGatewayWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	controlGateway := &productClaudeCodeControlGatewayFixture{}
	gateway, err := newProductHarnessGatewayConversationResponder(
		productHarnessGatewayConversationConfig{
			Executor: router, WorkspacePath: workspace,
			Events: &productHarnessGatewayEventFixture{},
			Now:    func() time.Time { return time.Unix(12, 0).UTC() },
			LoomNativeSegment: &productLoomNativeSegmentBackendConfig{
				Responder: router, ControlRegistry: registry,
				ControlGateway: controlGateway,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gateway.Close(context.Background()) })
	request := api.LocalProductConversationRequest{
		ThreadID: "conversation-native", SegmentID: "segment-native",
		AttemptID: "attempt-native-1", IncidentID: "incident-native-1",
		ProfileID: profileID, ModelID: provider.DeepSeekConversationModelID,
		ExecutionBinding:            &binding,
		ContextPrompt:               "capsule prompt",
		ContextCapsuleDigest:        productHarnessGatewayDigest("capsule:native"),
		SegmentContextCapsuleDigest: productHarnessGatewayDigest("capsule:native"),
		SegmentBindingDigest:        productHarnessGatewayDigest("binding:native"),
		BindingDigest:               productHarnessGatewayDigest("attempt:native"),
		CatalogDigest:               controltool.DigestBytes([]byte("catalog")),
		Messages: []api.LocalProductChatMessage{{
			MessageID: "message-native-1", SegmentID: "segment-native",
			Role: "user", Content: "Create a Mission to ship the release",
		}},
	}
	response, err := gateway.Respond(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if response.Content != "I prepared a governed Mission proposal with Loom Native." ||
		len(response.ActionProposals) != 1 ||
		response.ActionProposals[0].ToolID != controltool.ToolMissionsCreatePreview ||
		response.ActionProposals[0].AttemptID != request.AttemptID ||
		controlGateway.calls != 2 || controlGateway.readCalls != 1 ||
		controlGateway.proposalCalls != 1 {
		t.Fatalf("Loom Native control response = %#v calls=%d", response, controlGateway.calls)
	}
	assertProductActionProposalFrozenTurn(
		t, response.ActionProposals[0], request, registry,
	)
	wantIdentity := credentialvault.CredentialIdentity{
		ProviderID: credential.ProviderID, ProviderAccountID: credential.ProviderAccountID,
		CredentialReference: credential.CredentialReference,
		CredentialRevision:  credential.Revision,
	}
	if leases.calls != 1 || leases.identity != wantIdentity ||
		providerClient.modelID != provider.DeepSeekConversationModelID ||
		providerClient.readToolName != "loom_workspace_status" ||
		providerClient.toolName != "loom_missions_create_preview" ||
		!providerClient.proposalStopsAfterSuccess ||
		providerClient.toolCount != len(registry.Definitions()) ||
		!providerClient.sawCredential {
		t.Fatalf(
			"lease=%#v calls=%d provider=%#v", leases.identity, leases.calls, providerClient,
		)
	}
}

func TestProductLoomNativeSegmentPreservesProviderFailureInGatewayDiagnostic(t *testing.T) {
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	credential := projection.ProviderCredentialRecord{
		ProviderID: "deepseek", ProviderAccountID: "deepseek.primary",
		CredentialReference: "credential-ref-deepseek-diagnostic",
		Revision:            9, Status: "verified", AuthMode: "brokered",
	}
	providerClient, err := provider.NewDeepSeekConversationClient(
		provider.DeepSeekConversationConfig{
			Client: profileConversationHTTPDoerFunc(func(request *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusPaymentRequired,
					Body: io.NopCloser(strings.NewReader(
						`{"error":{"code":"insufficient_balance","message":"must never escape"}}`,
					)),
					Request: request,
				}, nil
			}),
			Timeout: time.Second, MaxResponseBytes: 4096,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	router, err := newProductConversationProfileRouter(
		nil,
		func(providerID string) []projection.ProviderCredentialRecord {
			if providerID == credential.ProviderID {
				return []projection.ProviderCredentialRecord{credential}
			}
			return nil
		},
		&profileConversationLeaseRecorder{},
		providerClient,
	)
	if err != nil {
		t.Fatal(err)
	}
	router.requireExecutionBinding = true
	profileID := provider.DeepSeekConversationAccountProfileID(
		credential.ProviderAccountID, credential.Revision,
	)
	binding, err := router.ResolveConversationExecutionBinding(
		context.Background(), profileID, provider.DeepSeekConversationModelID,
	)
	if err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(t.TempDir(), "workspace")
	if err := prepareProductHarnessGatewayWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	diagnostics := &productConversationCompositionDiagnosticFixture{}
	events := &productHarnessGatewayOperationalEventSink{diagnostics: diagnostics}
	gateway, err := newProductHarnessGatewayConversationResponder(
		productHarnessGatewayConversationConfig{
			Executor: router, WorkspacePath: workspace, Events: events,
			Now: func() time.Time { return time.Unix(13, 0).UTC() },
			LoomNativeSegment: &productLoomNativeSegmentBackendConfig{
				Responder: router, ControlRegistry: registry,
				ControlGateway: &productClaudeCodeControlGatewayFixture{},
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gateway.Close(context.Background()) })
	request := api.LocalProductConversationRequest{
		ThreadID: "conversation-native-failure", SegmentID: "segment-native-failure",
		AttemptID: "attempt-native-failure", IncidentID: "incident-native-failure",
		ProfileID: profileID, ModelID: provider.DeepSeekConversationModelID,
		ExecutionBinding:            &binding,
		ContextCapsuleDigest:        productHarnessGatewayDigest("capsule:native-failure"),
		SegmentContextCapsuleDigest: productHarnessGatewayDigest("capsule:native-failure"),
		SegmentBindingDigest:        productHarnessGatewayDigest("binding:native-failure"),
		BindingDigest:               productHarnessGatewayDigest("attempt:native-failure"),
		CatalogDigest:               controltool.DigestBytes([]byte("catalog:native-failure")),
		Messages: []api.LocalProductChatMessage{{
			MessageID: "message-native-failure", SegmentID: "segment-native-failure",
			Role: "user", Content: "Create a Mission to ship the release",
		}},
	}
	_, err = gateway.Respond(context.Background(), request)
	failure, ok := api.LocalProductConversationDispatchFailureDetails(err)
	if !ok || failure.Code != "provider_insufficient_balance" ||
		failure.Stage != "provider_http" || failure.HTTPStatus != http.StatusPaymentRequired ||
		failure.ProviderCode != "insufficient_balance" || failure.Retryable ||
		strings.Contains(err.Error(), "must never escape") {
		t.Fatalf("failure=%#v ok=%t error=%v", failure, ok, err)
	}
	var terminal *productOperationalDiagnosticRecord
	for index := range diagnostics.records {
		if diagnostics.records[index].GatewayEventType == string(harnessgateway.EventResponseFailed) {
			terminal = &diagnostics.records[index]
			break
		}
	}
	if terminal == nil || terminal.IncidentID != request.IncidentID ||
		terminal.Stage != failure.Stage || terminal.ErrorCode != failure.Code ||
		terminal.HTTPStatus != failure.HTTPStatus ||
		terminal.ProviderErrorCode != failure.ProviderCode ||
		terminal.Retryable != failure.Retryable || terminal.RetryAfterSeconds != 0 {
		t.Fatalf("terminal diagnostic=%#v, want failure=%#v", terminal, failure)
	}
}

type productLoomNativeControlProviderFixture struct {
	modelID                   string
	readToolName              string
	toolName                  string
	toolCount                 int
	sawCredential             bool
	proposalStopsAfterSuccess bool
}

func (*productLoomNativeControlProviderFixture) Respond(
	context.Context,
	[]provider.ConversationMessage,
	[]byte,
) (string, error) {
	return "", errors.New("plain Provider path must not be used")
}

func (*productLoomNativeControlProviderFixture) RespondConfigured(
	context.Context,
	[]provider.ConversationMessage,
	[]byte,
	string,
	string,
) (string, error) {
	return "", errors.New("non-tool Provider path must not be used")
}

func (client *productLoomNativeControlProviderFixture) RespondConfiguredWithTools(
	ctx context.Context,
	_ []provider.ConversationMessage,
	secret []byte,
	modelID string,
	_ string,
	tools []provider.ConversationControlTool,
	execute provider.ConversationControlExecutor,
) (string, error) {
	client.modelID = modelID
	client.toolCount = len(tools)
	client.sawCredential = string(secret) == "account-private-key"
	found := false
	foundRead := false
	for _, tool := range tools {
		if tool.Name == "loom_missions_create_preview" {
			found = true
			client.proposalStopsAfterSuccess = tool.StopAfterSuccess
		}
		if tool.Name == "loom_workspace_status" {
			if tool.StopAfterSuccess {
				return "", errors.New("read Tool cannot stop the Provider loop")
			}
			foundRead = true
		}
	}
	if !found || !foundRead || execute == nil {
		return "", errors.New("frozen Mission tool missing")
	}
	client.readToolName = "loom_workspace_status"
	readResult, err := execute(ctx, client.readToolName, json.RawMessage(`{}`))
	if err != nil || !strings.Contains(string(readResult), `"workspace_id":"workspace-test"`) {
		return "", errors.New("bounded workspace metadata unavailable")
	}
	client.toolName = "loom_missions_create_preview"
	result, err := execute(
		ctx, client.toolName,
		json.RawMessage(`{"objective":"Ship the governed release"}`),
	)
	if err != nil || string(result) != `{"requires_confirmation":true}` {
		return "", errors.New("governed Mission proposal unavailable")
	}
	return "I prepared a governed Mission proposal with Loom Native.", nil
}
