package harnessadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/controltool"
)

func TestHarnessControlMCPAdvertisesTypedToolsAndCapturesProposal(t *testing.T) {
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Date(2026, 8, 28, 11, 0, 0, 0, time.UTC)
	route, workspace := phase7ControlTurnReferences()
	proposal, err := controltool.NewSessionAlignmentProposal(
		controltool.SessionAlignmentProposalInput{
			ProposalID: "proposal-1", TargetConversationID: "s3",
			TargetContentDigest: controltool.DigestBytes([]byte("target")),
			Sources: []controltool.SessionSource{{
				ConversationID: "s1", Title: "Session one",
				ContentDigest: controltool.DigestBytes([]byte("source")), MessageCount: 2,
			}},
			ContextMode:   controltool.ContextModeSummaryOnly,
			CatalogDigest: controltool.DigestBytes([]byte("catalog")),
			Route:         route, Workspace: workspace, RegistryDigest: registry.Digest(),
			IncidentID: "incident-1",
			SegmentID:  "segment-1", AttemptID: "attempt-1",
			CreatedAt: expires.Add(-time.Minute), ExpiresAt: expires,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	actionProposal, err := controltool.NewConversationActionProposal(
		controltool.ConversationActionProposalInput{
			ProposalID: "proposal-mission-1", ToolID: controltool.ToolMissionsCreatePreview,
			TargetConversationID: "s3",
			TargetContentDigest:  controltool.DigestBytes([]byte("target")),
			Argument:             "Ship the governed release", SegmentID: "segment-1",
			Route: route, Workspace: workspace, RegistryDigest: registry.Digest(),
			IncidentID: "incident-1",
			AttemptID:  "attempt-1", CreatedAt: expires.Add(-time.Minute),
			ExpiresAt: expires,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	gateway := &controlMCPGatewayStub{proposal: proposal, actionProposal: actionProposal}
	service, err := newHarnessControlMCP(context.Background(), registry, gateway)
	if err != nil {
		t.Fatalf("newHarnessControlMCP() error = %v", err)
	}
	defer service.Close()
	if got, want := service.Lease().ToolNames, []string{
		"loom_sessions_search",
		"loom_sessions_align_preview",
		"loom_missions_create_preview",
		"loom_missions_continue_preview",
		"loom_teams_create_preview",
		"loom_roundtables_open_preview",
		"loom_missions_search",
		"loom_missions_status",
		"loom_teams_search",
		"loom_teams_status",
		"loom_roundtables_status",
		"loom_governance_needs_you",
		"loom_runtimes_status",
		"loom_providers_status",
		"loom_diagnostics_incident",
		"loom_workspace_status",
		"loom_conversation_route_status",
		"loom_library_search",
		"loom_conversation_route_change_preview",
		"loom_conversation_model_change_preview",
		"loom_conversation_reasoning_change_preview",
		"loom_workspace_choose_preview",
		"loom_teams_edit_preview",
		"loom_roundtables_pause_preview",
		"loom_roundtables_steer_preview",
		"loom_roundtables_retry_preview",
		"loom_roundtables_skip_preview",
		"loom_roundtables_replace_preview",
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("control lease tool names = %#v, want %#v", got, want)
	}
	if err := service.BeginTurn(controltool.TurnContext{
		ConversationID: "s3", SegmentID: "segment-1", AttemptID: "attempt-1",
		IncidentID: "incident-1", CatalogDigest: controltool.DigestBytes([]byte("catalog")),
	}); err != nil {
		t.Fatalf("BeginTurn() error = %v", err)
	}

	listBody := []byte(`{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}`)
	response, status := service.handle(context.Background(), listBody)
	if status != 200 {
		t.Fatalf("tools/list status = %d, body = %s", status, response)
	}
	var listed struct {
		Result struct {
			Tools []struct {
				Name        string         `json:"name"`
				InputSchema map[string]any `json:"inputSchema"`
				Annotations map[string]any `json:"annotations"`
			} `json:"tools"`
		} `json:"result"`
	}
	if json.Unmarshal(response, &listed) != nil ||
		len(listed.Result.Tools) != len(registry.Definitions()) {
		t.Fatalf("tools/list response = %s", response)
	}
	if listed.Result.Tools[0].Name != "loom_sessions_search" ||
		listed.Result.Tools[1].Name != "loom_sessions_align_preview" ||
		listed.Result.Tools[0].InputSchema["additionalProperties"] != false ||
		listed.Result.Tools[1].Annotations["readOnlyHint"] != false {
		t.Fatalf("tools/list definitions = %#v", listed.Result.Tools)
	}

	callBody := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"loom_sessions_align_preview","arguments":{"session_ids":["s1"],"context_mode":"summary_only"}}}`)
	response, status = service.handle(context.Background(), callBody)
	if status != 200 || !bytes.Contains(response, []byte(`"requires_confirmation":true`)) {
		t.Fatalf("tools/call status = %d, body = %s", status, response)
	}
	if gateway.calls != 1 || gateway.lastCall.ToolID != controltool.ToolSessionsAlignPreview ||
		gateway.lastTurn.ConversationID != "s3" ||
		gateway.lastTurn.RegistryDigest != registry.Digest() {
		t.Fatalf("gateway call = %#v turn = %#v", gateway.lastCall, gateway.lastTurn)
	}
	alignmentBatch, err := service.EndTurn()
	if err != nil || len(alignmentBatch.SessionAlignments) != 1 ||
		len(alignmentBatch.ConversationActions) != 0 ||
		alignmentBatch.SessionAlignments[0].ProposalDigest != proposal.ProposalDigest ||
		!reflect.DeepEqual(alignmentBatch.CompletedCalls, []controltool.CompletedCall{{
			ToolID: controltool.ToolSessionsAlignPreview, ToolVersion: 2,
			Effect: controltool.EffectProposal,
		}}) {
		t.Fatalf("alignment EndTurn() = %#v, %v", alignmentBatch, err)
	}
	if err := service.BeginTurn(controltool.TurnContext{
		ConversationID: "s3", SegmentID: "segment-1", AttemptID: "attempt-1",
		IncidentID: "incident-1", CatalogDigest: controltool.DigestBytes([]byte("catalog")),
	}); err != nil {
		t.Fatal(err)
	}
	actionCall := []byte(`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"loom_missions_create_preview","arguments":{"objective":"Ship the governed release"}}}`)
	response, status = service.handle(context.Background(), actionCall)
	if status != 200 || !bytes.Contains(response, []byte(`"requires_confirmation":true`)) {
		t.Fatalf("action tools/call status = %d, body = %s", status, response)
	}
	batch, err := service.EndTurn()
	if err != nil || len(batch.SessionAlignments) != 0 || len(batch.ConversationActions) != 1 ||
		batch.ConversationActions[0].ProposalDigest != actionProposal.ProposalDigest ||
		!reflect.DeepEqual(batch.CompletedCalls, []controltool.CompletedCall{
			{ToolID: controltool.ToolMissionsCreatePreview, ToolVersion: 2, Effect: controltool.EffectProposal},
		}) {
		t.Fatalf("EndTurn() = %#v, %v", batch, err)
	}
	if err := service.BeginTurn(controltool.TurnContext{
		ConversationID: "s3", SegmentID: "segment-1", AttemptID: "attempt-2",
		IncidentID: "incident-2", CatalogDigest: controltool.DigestBytes([]byte("catalog")),
	}); err != nil {
		t.Fatal(err)
	}
	empty, err := service.EndTurn()
	if err != nil || len(empty.CompletedCalls) != 0 {
		t.Fatalf("completed calls leaked across turns: %#v, %v", empty, err)
	}
	if _, status := service.handle(context.Background(), callBody); status != 200 {
		t.Fatalf("post-turn call status = %d, want JSON-RPC error response", status)
	}
}

func phase7ControlTurnReferences() (
	*controltool.FrozenRouteReference,
	*controltool.FrozenWorkspaceReference,
) {
	return &controltool.FrozenRouteReference{
			HarnessAdapter: "codex", ProviderID: "openai",
			ProviderAccountID: "openai.primary", CredentialRevision: 3,
			ModelID: "gpt-5.6-sol", ReasoningEffort: "max",
			ExecutionBindingDigest: controltool.DigestBytes([]byte("binding")),
			ContextCapsuleDigest:   controltool.DigestBytes([]byte("capsule")),
		}, &controltool.FrozenWorkspaceReference{
			WorkspaceID:     "workspace-primary",
			WorkspaceDigest: controltool.DigestBytes([]byte("workspace")),
		}
}

func TestCallHarnessControlToolUsesAuthenticatedTurnPathAndRejectsResultDrift(t *testing.T) {
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	gateway := &controlMCPGatewayStub{}
	server, err := OpenHarnessControlServer(context.Background(), registry, gateway)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	if err := server.BeginTurn(controltool.TurnContext{
		ConversationID: "conversation-1", SegmentID: "segment-1", AttemptID: "attempt-1",
		IncidentID: "incident-1", CatalogDigest: controltool.DigestBytes([]byte("catalog")),
	}); err != nil {
		t.Fatal(err)
	}
	if err := VerifyHarnessControlToolCompletions(
		context.Background(), server.Lease(), []string{"loom_sessions_search"},
	); !errors.Is(err, ErrHarnessControlMCP) {
		t.Fatalf("unobserved completion verification error = %v", err)
	}
	result, err := CallHarnessControlTool(
		context.Background(), server.Lease(), "loom_sessions_search",
		json.RawMessage(`{"query":"session"}`),
	)
	if err != nil || string(result) != `{"sessions":[]}` || gateway.calls != 1 ||
		gateway.lastCall.ToolID != controltool.ToolSessionsSearch {
		t.Fatalf("result=%s calls=%d call=%#v error=%v", result, gateway.calls, gateway.lastCall, err)
	}
	if err := VerifyHarnessControlToolCompletions(
		context.Background(), server.Lease(), []string{"loom_sessions_search"},
	); err != nil {
		t.Fatalf("observed completion verification error = %v", err)
	}
	result, err = CallHarnessControlTool(
		context.Background(), server.Lease(), "loom_sessions_search",
		json.RawMessage(`{"query":"session again"}`),
	)
	if err != nil || string(result) != `{"sessions":[]}` || gateway.calls != 2 {
		t.Fatalf("repeated result=%s calls=%d error=%v", result, gateway.calls, err)
	}
	if err := VerifyHarnessControlToolCompletions(
		context.Background(), server.Lease(),
		[]string{"loom_sessions_search", "loom_sessions_search"},
	); err != nil {
		t.Fatalf("repeated completion verification error = %v", err)
	}
	if err := VerifyHarnessControlToolCompletions(
		context.Background(), server.Lease(), []string{"loom_runtimes_status"},
	); !errors.Is(err, ErrHarnessControlMCP) {
		t.Fatalf("substituted completion verification error = %v", err)
	}
	if _, err := server.EndTurn(); err != nil {
		t.Fatal(err)
	}
	if err := VerifyHarnessControlToolCompletions(
		context.Background(), server.Lease(),
		[]string{"loom_sessions_search", "loom_sessions_search"},
	); !errors.Is(err, ErrHarnessControlMCP) {
		t.Fatalf("ended-turn completion verification error = %v", err)
	}

	const token = "0000000000000000000000000000000000000000000000000000000000000000"
	driftServer := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, readErr := io.ReadAll(request.Body)
		if readErr != nil || request.Header.Get("Authorization") != "Bearer "+token ||
			strings.Contains(string(body), token) {
			t.Errorf("request authorization/body = %q / %s", request.Header.Get("Authorization"), body)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"content":[{"type":"text","text":"{\"sessions\":[]}"}],"structuredContent":{"sessions":[{"id":"drift"}]},"isError":false}}`))
	}))
	defer driftServer.Close()
	lease := HarnessControlMCPLease{
		URL: driftServer.URL + "/mcp", Token: token,
		ToolNames: []string{"loom_sessions_search"},
	}
	if _, err := CallHarnessControlTool(
		context.Background(), lease, "loom_sessions_search", json.RawMessage(`{}`),
	); !errors.Is(err, ErrHarnessControlMCP) {
		t.Fatalf("drift error = %v", err)
	}
	lease.URL += "?redirect=true"
	if _, err := CallHarnessControlTool(
		context.Background(), lease, "loom_sessions_search", json.RawMessage(`{}`),
	); !errors.Is(err, ErrHarnessControlMCP) {
		t.Fatalf("query-bearing lease error = %v", err)
	}
}

func TestCallHarnessControlToolPreservesSafeGatewayFailureClass(t *testing.T) {
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	gateway := &controlMCPGatewayStub{err: controltool.ErrInvalidCall}
	server, err := OpenHarnessControlServer(context.Background(), registry, gateway)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	if err := server.BeginTurn(controltool.TurnContext{
		ConversationID: "conversation-1", SegmentID: "segment-1", AttemptID: "attempt-1",
		CatalogDigest: controltool.DigestBytes([]byte("catalog")),
	}); err != nil {
		t.Fatal(err)
	}
	_, err = CallHarnessControlTool(
		context.Background(), server.Lease(), "loom_missions_create_preview",
		json.RawMessage(`{"objective":"P7","title":"undeclared"}`),
	)
	if !errors.Is(err, ErrHarnessControlMCP) ||
		HarnessControlCallFailureCode(err) != harnessControlCallFailureInvalidRequest {
		t.Fatalf("gateway failure = %v code=%s", err, HarnessControlCallFailureCode(err))
	}
}

func TestHarnessControlMCPRetriesOneTransientReadGatewayFailureOnly(t *testing.T) {
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	readGateway := &controlMCPGatewayStub{
		err: controltool.ErrGatewayUnavailable, failures: 1,
	}
	readServer, err := OpenHarnessControlServer(context.Background(), registry, readGateway)
	if err != nil {
		t.Fatal(err)
	}
	defer readServer.Close()
	if err := readServer.BeginTurn(controltool.TurnContext{
		ConversationID: "conversation-read", SegmentID: "segment-1", AttemptID: "attempt-1",
		CatalogDigest: controltool.DigestBytes([]byte("catalog")),
	}); err != nil {
		t.Fatal(err)
	}
	result, err := CallHarnessControlTool(
		context.Background(), readServer.Lease(), "loom_sessions_search",
		json.RawMessage(`{"query":"session"}`),
	)
	if err != nil || string(result) != `{"sessions":[]}` || readGateway.calls != 2 {
		t.Fatalf("read result=%s calls=%d error=%v", result, readGateway.calls, err)
	}
	batch, err := readServer.EndTurn()
	if err != nil || len(batch.CompletedCalls) != 1 ||
		batch.CompletedCalls[0].ToolID != controltool.ToolSessionsSearch {
		t.Fatalf("read completion=%#v error=%v", batch.CompletedCalls, err)
	}

	proposalGateway := &controlMCPGatewayStub{
		err: controltool.ErrGatewayUnavailable, failures: 1,
	}
	proposalServer, err := OpenHarnessControlServer(
		context.Background(), registry, proposalGateway,
	)
	if err != nil {
		t.Fatal(err)
	}
	defer proposalServer.Close()
	if err := proposalServer.BeginTurn(controltool.TurnContext{
		ConversationID: "conversation-proposal", SegmentID: "segment-1", AttemptID: "attempt-1",
		CatalogDigest: controltool.DigestBytes([]byte("catalog")),
	}); err != nil {
		t.Fatal(err)
	}
	_, err = CallHarnessControlTool(
		context.Background(), proposalServer.Lease(), "loom_missions_create_preview",
		json.RawMessage(`{"objective":"P7"}`),
	)
	if !errors.Is(err, ErrHarnessControlMCP) ||
		HarnessControlCallFailureCode(err) != harnessControlCallFailureToolUnavailable ||
		proposalGateway.calls != 1 {
		t.Fatalf("proposal calls=%d code=%s error=%v", proposalGateway.calls, HarnessControlCallFailureCode(err), err)
	}
}

func TestHarnessControlMCPFailsClosedWithoutFrozenTurn(t *testing.T) {
	registry, _ := controltool.NewBuiltinRegistry()
	service, err := newHarnessControlMCP(
		context.Background(), registry, &controlMCPGatewayStub{},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	call := []byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"loom_sessions_search","arguments":{"query":"s1"}}}`)
	response, status := service.handle(context.Background(), call)
	if status != 200 || !bytes.Contains(response, []byte(`turn_unavailable`)) {
		t.Fatalf("call without turn = %d %s", status, response)
	}
	if err := service.BeginTurn(controltool.TurnContext{
		ConversationID: "s3", SegmentID: "segment-1", AttemptID: "attempt-1",
		CatalogDigest: controltool.DigestBytes([]byte("catalog")),
	}); err != nil {
		t.Fatal(err)
	}
	if err := service.BeginTurn(controltool.TurnContext{
		ConversationID: "s3", SegmentID: "segment-1", AttemptID: "attempt-2",
		CatalogDigest: controltool.DigestBytes([]byte("catalog")),
	}); !errors.Is(err, ErrHarnessControlMCP) {
		t.Fatalf("overlapping BeginTurn() error = %v", err)
	}
}

func TestHarnessControlMCPPublishesTerminalProposalOnlyAfterAcknowledgementWrite(t *testing.T) {
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		t.Fatal(err)
	}
	expires := time.Date(2026, 8, 31, 11, 0, 0, 0, time.UTC)
	route, workspace := phase7ControlTurnReferences()
	proposal, err := controltool.NewConversationActionProposal(
		controltool.ConversationActionProposalInput{
			ProposalID: "proposal-terminal-1", ToolID: controltool.ToolMissionsCreatePreview,
			TargetConversationID: "conversation-terminal",
			TargetContentDigest:  controltool.DigestBytes([]byte("target")),
			Argument:             "Ship the governed release", SegmentID: "segment-terminal",
			Route: route, Workspace: workspace, RegistryDigest: registry.Digest(),
			IncidentID: "incident-terminal", AttemptID: "attempt-terminal",
			CreatedAt: expires.Add(-time.Minute), ExpiresAt: expires,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	server, err := OpenHarnessControlServer(
		context.Background(), registry, &controlMCPGatewayStub{actionProposal: proposal},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	if err := server.BeginTurn(controltool.TurnContext{
		ConversationID: "conversation-terminal", SegmentID: "segment-terminal",
		AttemptID: "attempt-terminal", IncidentID: "incident-terminal",
		CatalogDigest: controltool.DigestBytes([]byte("catalog")),
		Route:         *route, Workspace: *workspace,
	}); err != nil {
		t.Fatal(err)
	}
	terminal, err := server.TerminalProposalCompleted()
	if err != nil {
		t.Fatal(err)
	}
	lease := server.Lease()
	request := httptest.NewRequest(
		http.MethodPost,
		lease.URL,
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"loom_missions_create_preview","arguments":{"objective":"Ship the governed release"}}}`),
	)
	request.Header.Set("Authorization", "Bearer "+lease.Token)
	request.Header.Set("Content-Type", "application/json")
	writer := newBlockingHarnessResponseWriter()
	done := make(chan struct{})
	go func() {
		server.service.ServeHTTP(writer, request)
		close(done)
	}()
	select {
	case <-writer.writeStarted:
	case <-time.After(time.Second):
		t.Fatal("terminal Proposal response did not reach the writer")
	}
	select {
	case <-terminal:
		t.Fatal("terminal Proposal published before its MCP response was written")
	case <-time.After(20 * time.Millisecond):
	}
	close(writer.allowWrite)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("terminal Proposal response did not finish")
	}
	select {
	case <-terminal:
		t.Fatal("terminal Proposal published before the Harness acknowledged its result")
	case <-time.After(20 * time.Millisecond):
	}

	ackRequest := httptest.NewRequest(
		http.MethodPost,
		lease.URL,
		strings.NewReader(`{"jsonrpc":"2.0","id":3,"method":"loom/control/acknowledged","params":{}}`),
	)
	ackRequest.Header.Set("Authorization", "Bearer "+lease.Token)
	ackRequest.Header.Set("Content-Type", "application/json")
	ackWriter := newBlockingHarnessResponseWriter()
	ackDone := make(chan struct{})
	go func() {
		server.service.ServeHTTP(ackWriter, ackRequest)
		close(ackDone)
	}()
	select {
	case <-ackWriter.writeStarted:
	case <-time.After(time.Second):
		t.Fatal("terminal acknowledgement did not reach the writer")
	}
	select {
	case <-terminal:
		t.Fatal("terminal Proposal published before its acknowledgement response was written")
	case <-time.After(20 * time.Millisecond):
	}
	close(ackWriter.allowWrite)
	select {
	case <-ackDone:
	case <-time.After(time.Second):
		t.Fatal("terminal acknowledgement response did not finish")
	}
	select {
	case <-terminal:
	case <-time.After(time.Second):
		t.Fatal("acknowledged terminal Proposal completion was not published")
	}

	response, status := server.service.handle(
		context.Background(),
		[]byte(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"loom_sessions_search","arguments":{"query":"later"}}}`),
	)
	if status != http.StatusOK || !bytes.Contains(response, []byte(`turn_unavailable`)) {
		t.Fatalf("post-Proposal control call = %d %s", status, response)
	}
	batch, err := server.EndTurn()
	if err != nil || len(batch.ConversationActions) != 1 ||
		len(batch.CompletedCalls) != 1 ||
		batch.CompletedCalls[0].Effect != controltool.EffectProposal {
		t.Fatalf("terminal Proposal batch = %#v, %v", batch, err)
	}
}

type blockingHarnessResponseWriter struct {
	header       http.Header
	writeStarted chan struct{}
	allowWrite   chan struct{}
	writeOnce    sync.Once
}

func newBlockingHarnessResponseWriter() *blockingHarnessResponseWriter {
	return &blockingHarnessResponseWriter{
		header: make(http.Header), writeStarted: make(chan struct{}),
		allowWrite: make(chan struct{}),
	}
}

func (writer *blockingHarnessResponseWriter) Header() http.Header { return writer.header }

func (*blockingHarnessResponseWriter) WriteHeader(int) {}

func (writer *blockingHarnessResponseWriter) Write(payload []byte) (int, error) {
	writer.writeOnce.Do(func() { close(writer.writeStarted) })
	<-writer.allowWrite
	return len(payload), nil
}

type controlMCPGatewayStub struct {
	proposal       controltool.SessionAlignmentProposal
	actionProposal controltool.ConversationActionProposal
	calls          int
	lastTurn       controltool.TurnContext
	lastCall       controltool.Call
	err            error
	failures       int
}

func (gateway *controlMCPGatewayStub) Call(
	_ context.Context,
	turn controltool.TurnContext,
	call controltool.Call,
) (controltool.Result, error) {
	gateway.calls++
	gateway.lastTurn = turn
	gateway.lastCall = call
	if gateway.err != nil && (gateway.failures == 0 || gateway.calls <= gateway.failures) {
		return controltool.Result{}, gateway.err
	}
	if call.ToolID == controltool.ToolSessionsAlignPreview {
		return controltool.Result{
			Content:  json.RawMessage(`{"requires_confirmation":true}`),
			Proposal: &gateway.proposal,
		}, nil
	}
	if call.ToolID == controltool.ToolMissionsCreatePreview {
		return controltool.Result{
			Content:        json.RawMessage(`{"requires_confirmation":true}`),
			ActionProposal: &gateway.actionProposal,
		}, nil
	}
	return controltool.Result{Content: json.RawMessage(`{"sessions":[]}`)}, nil
}
