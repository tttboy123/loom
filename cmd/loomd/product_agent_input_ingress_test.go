package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"loom-pi-rebuild/internal/agentinbox"
	"loom-pi-rebuild/internal/localipc"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/work"
)

func TestProductAgentInputIngressAdmitsExactActiveAttemptIdempotently(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	runs, run, executionBinding, capsule, _, _ := productAttemptLoopFixture(t)
	payloads, err := work.NewAttemptPayloadAuthority(runs)
	if err != nil {
		t.Fatal(err)
	}
	loops, err := work.NewAttemptLoopAuthority(runs, payloads)
	if err != nil {
		t.Fatal(err)
	}
	inboxAuthority, err := work.NewAgentInboxAuthority(loops)
	if err != nil {
		t.Fatal(err)
	}
	inboxStore := newProductMemoryAgentInboxStore()
	inbox, err := work.NewAgentInboxCoordinator(inboxAuthority, inboxStore)
	if err != nil {
		t.Fatal(err)
	}
	registry := newProductActiveAttemptRegistry()
	request := productAttemptLoopRequest(t, run, executionBinding, capsule)
	binding, budget, err := productAttemptLoopBinding(request)
	if err != nil {
		t.Fatal(err)
	}
	budget.MaxTurns = productAttemptLoopMaxInputTurns
	budget.MaxStepsPerTurn = productAttemptLoopMaxInputSteps
	turnID, stepID, _ := productAttemptLoopIDs(binding)
	if _, err := loops.StartTurn(ctx, binding, budget, work.AttemptLoopTurnInput{
		TurnID: turnID, Sequence: 1, InputDigest: productAttemptLoopDigest("turn"),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := loops.StartStep(ctx, binding, work.AttemptLoopStepInput{
		TurnID: turnID, StepID: stepID, Sequence: 1,
		ModelInputDigest: productAttemptLoopDigest("step"),
	}); err != nil {
		t.Fatal(err)
	}
	request.AgentInputs = productAgentInputSourceNoop{}
	registration, active, err := registry.Register(request, binding, budget)
	if err != nil {
		t.Fatal(err)
	}
	defer registration.Close()
	ingress, err := newProductAgentInputIngress(
		registry, inbox, productNativeAgentInputDiagnostics{},
		func() time.Time { return time.Date(2026, 8, 14, 15, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}

	content := []byte("steer toward the accepted boundary")
	command := productAgentInputRequest{
		SchemaVersion: 1, SegmentID: active.Identity.SegmentID,
		AgentInstanceID: active.Identity.AgentInstanceID,
		WorkItemID:      active.Identity.WorkItemID, RunID: active.Identity.RunID,
		ClaimGeneration: active.Identity.ClaimGeneration,
		Mode:            agentinbox.ModeSteer, ContextScope: agentinbox.ScopeAgentPrivate,
		Content: content, IncidentID: "agent-input-incident-1",
	}
	receipt, err := ingress.AdmitAgentInput(ctx, command)
	if err != nil {
		t.Fatal(err)
	}
	if receipt.SchemaVersion != 1 || receipt.IncidentID != command.IncidentID ||
		receipt.Mode != command.Mode || receipt.InputID == "" || receipt.OrderKey != 1 ||
		receipt.TargetStepSequence != 2 || receipt.TargetStepID == "" ||
		receipt.TargetTurnID != "" || receipt.TargetTurnSequence != 0 {
		t.Fatalf("receipt = %#v", receipt)
	}
	if !allProductAgentInputBytesZero(content) {
		t.Fatal("ingress did not clear caller-owned plaintext")
	}
	replayContent := []byte("steer toward the accepted boundary")
	replayed, err := ingress.AdmitAgentInput(ctx, productAgentInputRequest{
		SchemaVersion: command.SchemaVersion, SegmentID: command.SegmentID,
		AgentInstanceID: command.AgentInstanceID, WorkItemID: command.WorkItemID,
		RunID: command.RunID, ClaimGeneration: command.ClaimGeneration,
		Mode: command.Mode, ContextScope: command.ContextScope,
		Content: replayContent, IncidentID: command.IncidentID,
	})
	if err != nil || replayed != receipt || !allProductAgentInputBytesZero(replayContent) {
		t.Fatalf("replay receipt/error/zeroized = %#v / %v / %v", replayed, err, allProductAgentInputBytesZero(replayContent))
	}
	substituted := command
	substituted.Content = []byte("different content")
	if _, err := ingress.AdmitAgentInput(ctx, substituted); !errors.Is(err, errProductAgentInputConflict) {
		t.Fatalf("content substitution error = %v", err)
	}
	if !allProductAgentInputBytesZero(substituted.Content) {
		t.Fatal("rejected plaintext was not cleared")
	}
	stale := command
	stale.Content = []byte("stale generation")
	stale.IncidentID = "agent-input-incident-2"
	stale.ClaimGeneration++
	if _, err := ingress.AdmitAgentInput(ctx, stale); !errors.Is(err, errProductActiveAttemptNotFound) {
		t.Fatalf("stale admission error = %v", err)
	}
}

func TestProductDaemonRoutesStrictAgentInputWithIncidentID(t *testing.T) {
	t.Parallel()
	stub := &productAgentInputRouteStub{}
	handler := newProductRouteHandler(productRouteServices{agentInput: stub})
	params, err := json.Marshal(productAgentInputRequest{
		SchemaVersion: 1, SegmentID: "segment-1", AgentInstanceID: "agent-1",
		WorkItemID: "work-1", RunID: "run-1", ClaimGeneration: 1,
		Mode: agentinbox.ModeQueue, ContextScope: agentinbox.ScopeAgentPrivate,
		Content: []byte("continue"),
	})
	if err != nil {
		t.Fatal(err)
	}
	response := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "agent-input-route-1", Method: "agent_input", Params: params,
	})
	if !response.OK || response.Error != nil || stub.request.IncidentID != "agent-input-route-1" ||
		!bytes.Equal(stub.request.Content, []byte("continue")) {
		t.Fatalf("response/request = %#v / %#v", response, stub.request)
	}
	var receipt productAgentInputReceipt
	if err := json.Unmarshal(response.Result, &receipt); err != nil || receipt.IncidentID != "agent-input-route-1" {
		t.Fatalf("receipt/error = %#v / %v", receipt, err)
	}

	invalid := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "agent-input-route-2", Method: "agent_input",
		Params: append(params[:len(params)-1], []byte(`,"extra":true}`)...),
	})
	if invalid.OK || invalid.Error == nil || invalid.Error.Code != "invalid_request" ||
		invalid.Error.Stage != "agent_input_admission" {
		t.Fatalf("invalid response = %#v", invalid)
	}
}

func TestProductAgentInputTraversesAuthenticatedLocalIPC(t *testing.T) {
	t.Parallel()
	root, err := os.MkdirTemp("/private/tmp", "loom-agent-input-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	stub := &productAgentInputRouteStub{}
	server, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath: filepath.Join(root, "loomd.sock"), EffectiveUID: os.Geteuid(),
		BuildID: "agent-input-ipc-fixture",
		Handler: localipc.HandlerFunc(newProductRouteHandler(productRouteServices{
			agentInput: stub,
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
	request := productAgentInputRequest{
		SchemaVersion: 1, SegmentID: "segment-1", AgentInstanceID: "agent-1",
		WorkItemID: "work-1", RunID: "run-1", ClaimGeneration: 1,
		Mode: agentinbox.ModeQueue, ContextScope: agentinbox.ScopeAgentPrivate,
		Content: []byte("continue through IPC"),
	}
	var receipt productAgentInputReceipt
	if err := client.Call(context.Background(), "agent_input", request, &receipt); err != nil {
		t.Fatal(err)
	}
	if receipt.IncidentID == "" || stub.request.IncidentID != receipt.IncidentID ||
		!bytes.Equal(stub.request.Content, request.Content) {
		t.Fatalf("receipt/request = %#v / %#v", receipt, stub.request)
	}
}

type productAgentInputSourceNoop struct{}

func (productAgentInputSourceNoop) NextAgentInput(
	context.Context, loomruntime.AgentInputCheckpoint,
) (loomruntime.AgentInputBatch, bool, error) {
	return loomruntime.AgentInputBatch{}, false, nil
}

type productAgentInputRouteStub struct {
	request productAgentInputRequest
}

func (stub *productAgentInputRouteStub) AdmitAgentInput(
	_ context.Context, request productAgentInputRequest,
) (productAgentInputReceipt, error) {
	stub.request = request
	return productAgentInputReceipt{
		SchemaVersion: 1, IncidentID: request.IncidentID,
		InputID: "input-1", Mode: request.Mode, OrderKey: 1,
	}, nil
}
