package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/localipc"
)

func TestProductToolRecoveryRoutePreviewsAndResolvesExactCandidate(t *testing.T) {
	candidate := execution.ToolRecoveryCandidate{
		SchemaVersion: 1, Status: execution.ToolRecoveryCandidateAvailable,
		CandidateDigest: productTestHex("a"), ExecutionID: "execution-1",
		JobID: "work-1", CallDigest: productTestHex("b"), Tool: "bash",
		Generation: 3, OperationID: "operation-1", IncidentID: "incident-original",
		RecoveryCode: "side_effect_unknown", RecoveryRequiredAt: "2026-08-14T20:00:00Z",
		AvailableActions: []execution.ToolRecoveryAction{execution.ToolRecoveryAbortAttempt},
	}
	authority := &productToolRecoveryAuthorityFixture{
		preview: execution.ToolRecoveryPreview{SchemaVersion: 1, Candidates: []execution.ToolRecoveryCandidate{candidate}},
		decision: execution.ToolRecoveryDecision{
			SchemaVersion: 1, DecisionID: "11111111-1111-4111-8111-111111111111",
			Action:          execution.ToolRecoveryAbortAttempt,
			CandidateDigest: candidate.CandidateDigest, ExecutionID: candidate.ExecutionID,
		},
	}
	service, err := newProductToolRecoveryRoute(authority)
	if err != nil {
		t.Fatal(err)
	}
	preview, err := service.RecoverToolCall(context.Background(), productToolRecoveryRequest{
		SchemaVersion: 1, Operation: "preview",
		IncidentID: "22222222-2222-4222-8222-222222222222",
	})
	if err != nil || preview.Operation != "preview" || len(preview.Candidates) != 1 ||
		preview.Candidates[0].CandidateDigest != candidate.CandidateDigest ||
		preview.Candidates[0].ExecutionID != candidate.ExecutionID {
		t.Fatalf("preview = %#v, %v", preview, err)
	}
	resolved, err := service.RecoverToolCall(context.Background(), productToolRecoveryRequest{
		SchemaVersion: 1, Operation: "resolve",
		DecisionID:  "11111111-1111-4111-8111-111111111111",
		PrincipalID: "local-user", Action: execution.ToolRecoveryAbortAttempt,
		CandidateDigest: candidate.CandidateDigest,
		IncidentID:      "33333333-3333-4333-8333-333333333333",
	})
	if err != nil || resolved.Decision == nil ||
		resolved.Decision.DecisionID != authority.decision.DecisionID ||
		authority.input.CorrelationID != "33333333-3333-4333-8333-333333333333" ||
		authority.input.PrincipalID != "local-user" {
		t.Fatalf("resolved = %#v input=%#v err=%v", resolved, authority.input, err)
	}
}

func TestProductToolRecoveryTraversesAuthenticatedLocalIPC(t *testing.T) {
	root, err := os.MkdirTemp("/private/tmp", "loom-tool-recovery-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	authority := &productToolRecoveryAuthorityFixture{
		preview: execution.ToolRecoveryPreview{SchemaVersion: 1},
	}
	route, err := newProductToolRecoveryRoute(authority)
	if err != nil {
		t.Fatal(err)
	}
	server, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath: filepath.Join(root, "loomd.sock"), EffectiveUID: os.Geteuid(),
		BuildID: "tool-recovery-ipc-fixture",
		Handler: localipc.HandlerFunc(newProductRouteHandler(productRouteServices{
			toolRecovery: route,
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
		t.Fatalf("server failed before ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not become ready")
	}
	defer func() {
		cancel()
		if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("server close: %v", err)
		}
	}()
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: filepath.Join(root, "loomd.sock"), Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	var response productToolRecoveryResponse
	if err := client.Call(
		context.Background(), "tool_recovery",
		productToolRecoveryRequest{SchemaVersion: 1, Operation: "preview"},
		&response,
	); err != nil {
		t.Fatal(err)
	}
	if response.Operation != "preview" || response.IncidentID == "" {
		t.Fatalf("response = %#v", response)
	}

	params, err := json.Marshal(productToolRecoveryRequest{
		SchemaVersion: 1, Operation: "preview",
	})
	if err != nil {
		t.Fatal(err)
	}
	invalid := newProductRouteHandler(productRouteServices{toolRecovery: route})(
		context.Background(), localipc.Request{
			Version: 1, RequestID: "66666666-6666-4666-8666-666666666666",
			Method: "tool_recovery",
			Params: append(params[:len(params)-1], []byte(`,"extra":true}`)...),
		},
	)
	if invalid.OK || invalid.Error == nil || invalid.Error.Code != "invalid_request" ||
		invalid.Error.Stage != "tool_recovery" {
		t.Fatalf("invalid response = %#v", invalid)
	}
}

func TestProductToolRecoveryRouteRejectsUnknownAndMalformedRequests(t *testing.T) {
	service, err := newProductToolRecoveryRoute(&productToolRecoveryAuthorityFixture{})
	if err != nil {
		t.Fatal(err)
	}
	validIncident := "44444444-4444-4444-8444-444444444444"
	for _, request := range []productToolRecoveryRequest{
		{SchemaVersion: 2, Operation: "preview", IncidentID: validIncident},
		{SchemaVersion: 1, Operation: "unknown", IncidentID: validIncident},
		{SchemaVersion: 1, Operation: "preview", DecisionID: "unexpected", IncidentID: validIncident},
		{SchemaVersion: 1, Operation: "resolve", DecisionID: "not-a-uuid", PrincipalID: "local-user", Action: execution.ToolRecoveryAbortAttempt, CandidateDigest: productTestHex("a"), IncidentID: validIncident},
		{SchemaVersion: 1, Operation: "resolve", DecisionID: "55555555-5555-4555-8555-555555555555", PrincipalID: "local-user", Action: "retry", CandidateDigest: productTestHex("a"), IncidentID: validIncident},
	} {
		if _, err := service.RecoverToolCall(context.Background(), request); !errors.Is(err, errProductInvalidToolRecoveryRequest) {
			t.Fatalf("request %#v error = %v", request, err)
		}
	}
}

type productToolRecoveryAuthorityFixture struct {
	preview  execution.ToolRecoveryPreview
	decision execution.ToolRecoveryDecision
	input    execution.ToolRecoveryDecisionInput
	err      error
}

func (fixture *productToolRecoveryAuthorityFixture) Preview(
	context.Context,
) (execution.ToolRecoveryPreview, error) {
	return fixture.preview, fixture.err
}

func (fixture *productToolRecoveryAuthorityFixture) Resolve(
	_ context.Context,
	input execution.ToolRecoveryDecisionInput,
) (execution.ToolRecoveryDecision, error) {
	fixture.input = input
	return fixture.decision, fixture.err
}

func productTestHex(character string) string {
	value := ""
	for len(value) < 64 {
		value += character
	}
	return value
}
