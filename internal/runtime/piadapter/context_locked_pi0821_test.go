//go:build darwin

package piadapter

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/contextcapsule"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/supervisor"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

func TestLockedPi0821ContextExtensionTwoTurnContract(t *testing.T) {
	if os.Getenv("LOOM_P2D_PI_CONTEXT_CONTRACT") != "1" {
		t.Skip("locked Pi Context contract gate closed")
	}
	for _, path := range []string{lockedPiSearchEntry, lockedPiResolvedEntry, lockedPiNodeExecutable} {
		if _, err := os.Lstat(path); err != nil {
			t.Skip("locked Pi Context contract component unavailable")
		}
	}

	content := []byte("bounded observed context from the exact encrypted Capsule item")
	proposal := contextcapsule.RetrievalProposal{
		ItemID: "diff-detail", ContentDigest: piContextDigest(content),
		ArtifactRef: "artifact:diff-1",
	}
	retriever := &piContextRetrieverFixture{
		want: proposal, content: content,
		item: contextcapsule.RetrievedItem{
			ItemID: proposal.ItemID, Kind: contextcapsule.KindArtifactReference,
			Trust: contextcapsule.TrustObserved, Scope: contextcapsule.ScopeArtifactScoped,
			Priority: contextcapsule.PriorityRetrievable, TokenCount: 8,
			ContentDigest: proposal.ContentDigest,
			SourceType:    contextcapsule.SourceObservation, SourceRef: "evidence:diff-1",
			ArtifactRef: proposal.ArtifactRef,
		},
	}
	serverState := &lockedPiContextServerState{proposal: proposal, content: string(content)}
	server := newLockedPiContextServer(t, serverState)
	defer server.Close()

	fixture := newPiRPCBridgeFixture(t, "success")
	metadataRunner, err := NewPiMetadataProcessRunner(PiMetadataProcessRunnerConfig{
		ExecutablePath:     lockedPiSearchEntry,
		IsolationRoot:      fixture.tempPath,
		RuntimeSearchPaths: append([]string(nil), lockedPiRuntimeSearchPaths...),
		Timeout:            15 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	probe, err := loomruntime.NewPiRuntimeProbe(loomruntime.PiRuntimeProbeConfig{
		ProbeID: "probe.pi.context-contract", InstanceID: fixture.binding.RuntimeInstanceID,
		DeviceID: "device.local", DisplayName: "Locked Pi Context Contract",
		Runner: metadataRunner,
	})
	if err != nil {
		t.Fatal(err)
	}
	observations, err := probe.ObserveRuntime(context.Background())
	if err != nil || len(observations) != 1 ||
		!containsPiContextCapability(observations[0].Instance.ObservedCapabilities) {
		t.Fatalf("locked Pi Context capability observation = %#v, error=%v", observations, err)
	}
	runtimeAdapter, err := NewPiRPCBridgeAdapter(PiRPCBridgeAdapterConfig{
		Execution: PiExecutionAdapterConfig{
			ExecutablePath: lockedPiSearchEntry, RuntimeInstanceID: fixture.binding.RuntimeInstanceID,
			RuntimeSearchPaths: append([]string(nil), lockedPiRuntimeSearchPaths...),
			CancelGrace:        3 * time.Second, Now: func() time.Time { return time.Now().UTC() },
			Random: rand.Reader,
		},
		ProviderID: piRPCProviderID, ModelID: piRPCModelID,
		BaseURL: server.URL + "/v1", MaxAssistantBytes: 16_384,
	})
	if err != nil {
		t.Fatal(err)
	}

	request := lockedPiContextRequest(t, fixture, retriever, proposal)
	delivery := request.ContextDelivery.(*piContextDeliveryFixture)
	result, err := runtimeAdapter.Execute(context.Background(), request)
	if err != nil {
		t.Fatalf("locked Pi Context Execute() error = %v", err)
	}
	serverState.mu.Lock()
	requests := serverState.requests
	serverFailure := serverState.failure
	serverState.mu.Unlock()
	if requests != 2 || serverFailure != "" || retriever.calls != 1 {
		t.Fatalf(
			"requests=%d broker_calls=%d server_failure=%q",
			requests, retriever.calls, serverFailure,
		)
	}
	if delivery.prepares != 1 || delivery.acks != 1 ||
		delivery.proof != attemptpayload.ProofHarnessFinalOutput ||
		delivery.payload.Status != attemptpayload.StatusDelivered {
		t.Fatalf("locked Pi delivery = %#v", delivery)
	}
	frames := result.InboundFrames()
	var answer []byte
	for _, frame := range frames {
		if bytes.Contains(frame.Payload(), content) {
			t.Fatal("retrieved Context content leaked to an authorized Bridge frame")
		}
		if frame.Type() == bridgev1.MessageEvent {
			var payload struct {
				Delta string `json:"delta"`
			}
			if json.Unmarshal(frame.Payload(), &payload) != nil {
				t.Fatal("invalid final assistant Event frame")
			}
			answer = append(answer, payload.Delta...)
		}
	}
	if string(answer) != "Used the exact bounded context." {
		t.Fatalf("final assistant answer = %q", answer)
	}
	accounting, ok := result.Accounting()
	if !ok || accounting.InputTokens != 12 || accounting.OutputTokens != 7 ||
		accounting.TotalTokens != 19 {
		t.Fatalf("combined two-round accounting = %#v, available=%t", accounting, ok)
	}
	for _, residue := range []string{".loom-context-", ".sock"} {
		entries, readErr := os.ReadDir(fixture.homePath)
		if readErr != nil {
			t.Fatal(readErr)
		}
		for _, entry := range entries {
			if strings.Contains(entry.Name(), residue) {
				t.Fatalf("private Context extension residue survived: %s", entry.Name())
			}
		}
	}
}

func containsPiContextCapability(capabilities []string) bool {
	for _, capability := range capabilities {
		if capability == loomruntime.CapabilityContextRetrieval {
			return true
		}
	}
	return false
}

func lockedPiContextRequest(
	t testing.TB,
	fixture *piRPCBridgeFixture,
	retriever contextcapsule.Retriever,
	proposal contextcapsule.RetrievalProposal,
) supervisor.AdapterRequest {
	t.Helper()
	request := fixture.request(t)
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID: "profile-pi-context", AdapterType: "pi-cli",
		ProviderID: piRPCProviderID, ModelID: piRPCModelID,
		AuthMode: loomruntime.AuthNative, Timeout: time.Minute,
		RequiredCapabilities: []string{loomruntime.CapabilityContextRetrieval},
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: fixture.binding.RuntimeInstanceID, DeviceID: "device.local",
		AdapterType: "pi-cli", DisplayName: "Locked Pi 0.82.1",
		ExecutableVersion: "0.82.1", Status: loomruntime.RuntimeOnline,
		ObservedCapabilities: []string{loomruntime.CapabilityContextRetrieval}, Capacity: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	request.ExecutionBinding, err = loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	capsule, err := contextcapsule.BuildRoleContextCapsule(
		contextcapsule.Target{
			ConversationID: "conversation-pi-context", TeamID: "team-pi-context",
			AgentID: fixture.binding.SenderAgentInstanceID, RoleID: "coder",
			ProviderID: piRPCProviderID, ModelID: piRPCModelID,
			AuthMode: string(loomruntime.AuthNative), ContextAdapterID: "context:pi-cli:v1",
			DisclosurePolicyID: "policy.local", DisclosurePolicyVersion: 1, TokenBudget: 4,
			ArtifactRefs: []string{proposal.ArtifactRef},
		},
		[]contextcapsule.ItemInput{
			{
				ItemID: "goal", Kind: contextcapsule.KindConversationGoal,
				Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeTeamShared,
				Priority: contextcapsule.PrioritySystem, TokenCount: 4, Required: true,
				Content: []byte("Use exact context."), SourceType: contextcapsule.SourceAuthority,
				SourceRef: "goal:phase-2d",
			},
			{
				ItemID: proposal.ItemID, Kind: contextcapsule.KindArtifactReference,
				Trust: contextcapsule.TrustObserved, Scope: contextcapsule.ScopeArtifactScoped,
				Priority: contextcapsule.PriorityRetrievable, TokenCount: 8,
				Content:    []byte("bounded observed context from the exact encrypted Capsule item"),
				SourceType: contextcapsule.SourceObservation, SourceRef: "evidence:diff-1",
				ArtifactRef: proposal.ArtifactRef,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := contextcapsule.RenderDispatchPayload(capsule)
	if err != nil {
		t.Fatal(err)
	}
	request.Dispatch = piTestFrame(
		t, fixture.binding, 1, bridgev1.MessageDispatch, payload,
	)
	request.ContextCapsule = capsule.AuthorityRecord()
	request.ContextRetriever = retriever
	request.ContextDelivery = &piContextDeliveryFixture{retriever: retriever}
	return request
}

type lockedPiContextServerState struct {
	mu       sync.Mutex
	requests int
	failure  string
	proposal contextcapsule.RetrievalProposal
	content  string
}

func newLockedPiContextServer(
	t testing.TB,
	state *lockedPiContextServerState,
) *httptest.Server {
	t.Helper()
	handler := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		state.mu.Lock()
		state.requests++
		sequence := state.requests
		state.mu.Unlock()
		if host, _, err := net.SplitHostPort(request.RemoteAddr); err != nil ||
			!net.ParseIP(host).IsLoopback() || request.Method != http.MethodPost ||
			request.URL.Path != "/v1/chat/completions" ||
			request.Header.Get("Authorization") != "Bearer loom-local-offline" {
			state.fail("request_boundary")
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		body, err := io.ReadAll(io.LimitReader(request.Body, 128<<10))
		if err != nil || len(body) == 0 || len(body) >= 128<<10 {
			state.fail("request_body")
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		var payload struct {
			Messages []struct {
				Role       string `json:"role"`
				ToolCallID string `json:"tool_call_id"`
				Content    any    `json:"content"`
			} `json:"messages"`
			Tools []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
		}
		if json.Unmarshal(body, &payload) != nil || len(payload.Tools) != 1 ||
			payload.Tools[0].Function.Name != "loom_read_context" {
			state.fail("tool_schema")
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		if sequence == 1 {
			if len(payload.Messages) != 2 {
				state.fail("first_round_messages")
				writer.WriteHeader(http.StatusBadRequest)
				return
			}
			writeLockedPiContextSSE(writer, []any{
				lockedPiContextRoleChunk("context-response-1"),
				lockedPiContextToolChunk(state.proposal),
				lockedPiContextFinishChunk("context-response-1", "tool_calls"),
				lockedPiContextUsageChunk("context-response-1", 5, 3),
			})
			return
		}
		if sequence != 2 || len(payload.Messages) != 4 ||
			payload.Messages[2].Role != "assistant" || payload.Messages[3].Role != "tool" ||
			payload.Messages[3].ToolCallID != "context-call-1" ||
			!bytes.Contains(body, []byte(state.content)) {
			state.fail("second_round_context")
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		writeLockedPiContextSSE(writer, []any{
			lockedPiContextRoleChunk("context-response-2"),
			lockedPiContextTextChunk("context-response-2", "Used the exact bounded context."),
			lockedPiContextFinishChunk("context-response-2", "stop"),
			lockedPiContextUsageChunk("context-response-2", 7, 4),
		})
	})
	server := httptest.NewUnstartedServer(handler)
	address, ok := server.Listener.Addr().(*net.TCPAddr)
	if !ok || !address.IP.IsLoopback() {
		t.Fatal("Pi Context fixture did not bind loopback")
	}
	server.Start()
	return server
}

func (state *lockedPiContextServerState) fail(reason string) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.failure == "" {
		state.failure = reason
	}
}

func writeLockedPiContextSSE(writer http.ResponseWriter, chunks []any) {
	writer.Header().Set("Content-Type", "text/event-stream")
	flusher, _ := writer.(http.Flusher)
	for _, chunk := range chunks {
		body, _ := json.Marshal(chunk)
		_, _ = writer.Write(append(append([]byte("data: "), body...), '\n', '\n'))
		flusher.Flush()
	}
	_, _ = writer.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()
}

func lockedPiContextRoleChunk(id string) any {
	return map[string]any{
		"id": id, "object": "chat.completion.chunk", "created": 1, "model": piRPCModelID,
		"choices": []any{map[string]any{
			"index": 0, "delta": map[string]any{"role": "assistant"}, "finish_reason": nil,
		}},
	}
}

func lockedPiContextToolChunk(proposal contextcapsule.RetrievalProposal) any {
	arguments, _ := json.Marshal(map[string]string{
		"item_id": proposal.ItemID, "content_digest": proposal.ContentDigest,
		"artifact_ref": proposal.ArtifactRef,
	})
	return map[string]any{
		"id": "context-response-1", "object": "chat.completion.chunk", "created": 1,
		"model": piRPCModelID,
		"choices": []any{map[string]any{
			"index": 0, "finish_reason": nil,
			"delta": map[string]any{"tool_calls": []any{map[string]any{
				"index": 0, "id": "context-call-1", "type": "function",
				"function": map[string]any{"name": "loom_read_context", "arguments": string(arguments)},
			}}},
		}},
	}
}

func lockedPiContextTextChunk(id, text string) any {
	return map[string]any{
		"id": id, "object": "chat.completion.chunk", "created": 1, "model": piRPCModelID,
		"choices": []any{map[string]any{
			"index": 0, "delta": map[string]any{"content": text}, "finish_reason": nil,
		}},
	}
}

func lockedPiContextFinishChunk(id, reason string) any {
	return map[string]any{
		"id": id, "object": "chat.completion.chunk", "created": 1, "model": piRPCModelID,
		"choices": []any{map[string]any{
			"index": 0, "delta": map[string]any{}, "finish_reason": reason,
		}},
	}
}

func lockedPiContextUsageChunk(id string, input, output int) any {
	return map[string]any{
		"id": id, "object": "chat.completion.chunk", "created": 1, "model": piRPCModelID,
		"choices": []any{},
		"usage": map[string]any{
			"prompt_tokens": input, "completion_tokens": output, "total_tokens": input + output,
		},
	}
}

func TestLockedPiContextFixtureUsesPrivatePaths(t *testing.T) {
	root := t.TempDir()
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || filepath.Clean(root) != root {
		t.Fatal("test private root is invalid")
	}
}
