package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/controltool"
	"loom-pi-rebuild/internal/runtime/piadapter"
)

func TestProductPiSegmentBackendReturnsRegistryActionProposal(t *testing.T) {
	root := t.TempDir()
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(root, "workspace")
	if err := prepareProductHarnessGatewayWorkspace(workspace); err != nil {
		t.Fatal(err)
	}
	responder := &productPiControlResponderFixture{}
	controlGateway := &productClaudeCodeControlGatewayFixture{}
	gateway, err := newProductHarnessGatewayConversationResponder(
		productHarnessGatewayConversationConfig{
			Executor: &productHarnessConversationResponderFixture{}, WorkspacePath: workspace,
			Events: &productHarnessGatewayEventFixture{},
			Now:    func() time.Time { return time.Unix(11, 0).UTC() },
			PiSegment: &productPiSegmentBackendConfig{
				Responder: responder, ControlRegistry: registry,
				ControlGateway: controlGateway,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gateway.Close(context.Background()) })
	request := api.LocalProductConversationRequest{
		ThreadID: "conversation-pi", SegmentID: "segment-pi",
		AttemptID: "attempt-pi-1", IncidentID: "incident-pi-1",
		ProfileID: "conversation-pi-local", ModelID: productPiConversationModelID,
		ExecutionBinding: &api.LocalProductConversationExecutionBinding{
			SchemaVersion: 4, HarnessAdapter: "pi",
			ProviderID: productPiConversationProviderID, ModelID: productPiConversationModelID,
		},
		ContextPrompt:               "capsule prompt",
		ContextCapsuleDigest:        productHarnessGatewayDigest("capsule:pi"),
		SegmentContextCapsuleDigest: productHarnessGatewayDigest("capsule:pi"),
		SegmentBindingDigest:        productHarnessGatewayDigest("binding:pi"),
		BindingDigest:               productHarnessGatewayDigest("attempt:pi"),
		CatalogDigest:               controltool.DigestBytes([]byte("catalog")),
		Messages: []api.LocalProductChatMessage{{
			MessageID: "message-pi-1", SegmentID: "segment-pi",
			Role: "user", Content: "Create a Mission to ship the release",
		}},
	}
	response, err := gateway.Respond(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if response.Content != "I prepared a governed Mission proposal with Pi." ||
		len(response.ActionProposals) != 1 ||
		response.ActionProposals[0].ToolID != controltool.ToolMissionsCreatePreview ||
		response.ActionProposals[0].AttemptID != request.AttemptID {
		t.Fatalf("Pi control response = %#v", response)
	}
	assertProductActionProposalFrozenTurn(
		t, response.ActionProposals[0], request, registry,
	)
	if responder.control.URL == "" || responder.control.Token == "" ||
		len(responder.control.Tools) != len(registry.Definitions()) ||
		controlGateway.readCalls != 1 || controlGateway.proposalCalls != 1 {
		t.Fatalf("Pi control config = %#v", responder.control)
	}
}

type productPiControlResponderFixture struct {
	control piadapter.PiRPCConversationControlConfig
}

func (responder *productPiControlResponderFixture) respondWithControl(
	ctx context.Context,
	_ api.LocalProductConversationRequest,
	control piadapter.PiRPCConversationControlConfig,
) (api.LocalProductConversationResponse, error) {
	responder.control = control
	if err := callProductHarnessControlMCP(
		ctx, control.URL, control.Token,
		"loom_workspace_status", json.RawMessage(`{}`),
		[]byte(`"workspace_id":"workspace-test"`),
	); err != nil {
		return api.LocalProductConversationResponse{}, err
	}
	if err := callProductHarnessControlMCP(
		ctx, control.URL, control.Token,
		"loom_missions_create_preview",
		json.RawMessage(`{"objective":"Ship the governed release"}`),
		[]byte(`"requires_confirmation":true`),
	); err != nil {
		return api.LocalProductConversationResponse{}, err
	}
	return api.LocalProductConversationResponse{
		Content: "I prepared a governed Mission proposal with Pi.", Tentative: true,
	}, nil
}
