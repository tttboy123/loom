//go:build unix

package piadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"testing"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/permissions"
)

type piToolSuspendingHookFixture struct {
	mu      sync.Mutex
	calls   int
	proof   attemptpayload.DeliveryProof
	acked   int
	binding ToolCallBinding
	result  ToolCallResult
}

type piToolPendingHookFixture struct {
	called chan struct{}
	once   sync.Once
}

type piToolReadHookFixture struct {
	content []byte
	result  ToolCallResult
	acked   int
	proof   attemptpayload.DeliveryProof
}

func (hook *piToolReadHookFixture) ExecuteToolCall(
	context.Context,
	ToolCallEnvelope,
	ToolCallBinding,
) (ToolCallResult, error) {
	return hook.result, nil
}

func (hook *piToolReadHookFixture) ReadToolCallResultContent(
	context.Context,
	ToolCallBinding,
	ToolCallResult,
) ([]byte, error) {
	return bytes.Clone(hook.content), nil
}

func (hook *piToolReadHookFixture) AcknowledgeToolCallResultWithProof(
	_ context.Context,
	_ ToolCallBinding,
	_ ToolCallResult,
	proof attemptpayload.DeliveryProof,
) error {
	hook.acked++
	hook.proof = proof
	return nil
}

func (hook *piToolPendingHookFixture) ExecuteToolCall(
	context.Context,
	ToolCallEnvelope,
	ToolCallBinding,
) (ToolCallResult, error) {
	hook.once.Do(func() { close(hook.called) })
	return ToolCallResult{
		Verdict: permissions.VerdictAsk, ApprovalID: "approval-tool-pending",
		ApprovalDigest: strings.Repeat("a", 64),
	}, nil
}

func (hook *piToolSuspendingHookFixture) ExecuteToolCall(
	_ context.Context,
	_ ToolCallEnvelope,
	binding ToolCallBinding,
) (ToolCallResult, error) {
	hook.mu.Lock()
	defer hook.mu.Unlock()
	hook.calls++
	hook.binding = binding
	if hook.calls == 1 {
		return ToolCallResult{
			Verdict: permissions.VerdictAsk, ApprovalID: "approval-tool-1",
			ApprovalDigest: strings.Repeat("a", 64),
		}, nil
	}
	hook.result = ToolCallResult{
		Verdict: permissions.VerdictAllow, ExecutionID: "execution-tool-1",
		ApprovalID: "approval-tool-1", ApprovalDigest: strings.Repeat("a", 64),
		ResultNote: "completed", ExitCode: 0,
		OutputDigest: strings.Repeat("b", 64),
		Delivery: &ToolCallDelivery{
			Binding: attemptpayload.Binding{
				PayloadID: "payload-tool-1",
				Scope: attemptpayload.Scope{
					ConversationID: "conversation-tool-1", WorkItemID: "work-tool-1",
					RunID: "run-tool-1", ClaimGeneration: 1,
					RuntimeInstanceID:      "runtime-tool-1",
					ExecutionBindingDigest: strings.Repeat("c", 64),
					CapsuleDigest:          strings.Repeat("d", 64),
				},
				CallID: "call-tool-1", Sequence: 1,
				ContentType: "application/json", ContentDigest: strings.Repeat("e", 64),
			},
		},
	}
	return hook.result, nil
}

func (hook *piToolSuspendingHookFixture) AcknowledgeToolCallResultWithProof(
	_ context.Context,
	binding ToolCallBinding,
	result ToolCallResult,
	proof attemptpayload.DeliveryProof,
) error {
	hook.mu.Lock()
	defer hook.mu.Unlock()
	hook.acked++
	hook.binding = binding
	hook.result = result
	hook.proof = proof
	return nil
}

func TestPiToolExtensionWaitsForAskAndReturnsContentFreeAllowedResult(t *testing.T) {
	hook := &piToolSuspendingHookFixture{}
	binding := ToolCallBinding{
		ConversationID: "conversation-tool-1", WorkItemID: "work-tool-1",
		RunID: "run-tool-1", ClaimGeneration: 1,
		RuntimeInstanceID: "runtime-tool-1", AgentInstanceID: "agent-tool-1",
		ExecutionBindingDigest: strings.Repeat("c", 64),
		CapsuleDigest:          strings.Repeat("d", 64), ClaimID: "claim-tool-1",
		IncidentID: "incident-tool-1", JourneyID: "incident-tool-1",
	}
	extension := &piToolExtension{
		capability: strings.Repeat("1", 32), hook: hook, binding: binding,
	}
	envelope := ToolCallEnvelope{
		JobID: "work-tool-1",
		Call: permissions.ProposedCall{
			Tool: permissions.ToolBash, Command: "printf private-tool-command",
		},
	}
	payload, err := json.Marshal(piToolExtensionRequest{
		SchemaVersion: 1, Capability: extension.capability, Envelope: envelope,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := extension.handle(context.Background(), payload)
	if hook.calls != 2 || bytes.Contains(response, []byte("private-tool-command")) ||
		bytes.Contains(response, []byte("approval-tool-1")) ||
		bytes.Contains(response, []byte(strings.Repeat("a", 64))) {
		t.Fatalf("calls=%d response=%s", hook.calls, response)
	}
	parsedEnvelope, result, err := decodePiToolExtensionResponse(response)
	if err != nil || parsedEnvelope.Call.Tool != permissions.ToolBash ||
		result.Verdict != permissions.VerdictAllow || result.ExecutionID != "execution-tool-1" ||
		result.ApprovalID != "" || result.ApprovalDigest != "" {
		t.Fatalf("response envelope=%#v result=%#v err=%v", parsedEnvelope, result, err)
	}
	if err := extension.Acknowledge(
		context.Background(), attemptpayload.ProofHarnessFinalOutput,
	); err != nil {
		t.Fatal(err)
	}
	if hook.acked != 1 || hook.proof != attemptpayload.ProofHarnessFinalOutput ||
		!reflect.DeepEqual(hook.binding, binding) {
		t.Fatalf("ack=%d proof=%q binding=%#v", hook.acked, hook.proof, hook.binding)
	}
}

func TestPiToolExtensionResolvesAndAcknowledgesSequentialCalls(t *testing.T) {
	hook := &piToolSuspendingHookFixture{}
	binding := ToolCallBinding{
		ConversationID: "conversation-tool-1", WorkItemID: "work-tool-1",
		RunID: "run-tool-1", ClaimGeneration: 1,
		RuntimeInstanceID: "runtime-tool-1", AgentInstanceID: "agent-tool-1",
		ExecutionBindingDigest: strings.Repeat("c", 64),
		CapsuleDigest:          strings.Repeat("d", 64), ClaimID: "claim-tool-1",
		IncidentID: "incident-tool-1", JourneyID: "incident-tool-1",
	}
	extension := &piToolExtension{
		capability: strings.Repeat("1", 32), hook: hook, binding: binding,
	}
	first := ToolCallEnvelope{
		JobID: "work-tool-1",
		Call:  permissions.ProposedCall{Tool: permissions.ToolBash, Command: "printf first"},
	}
	second := ToolCallEnvelope{
		JobID: "work-tool-1",
		Call:  permissions.ProposedCall{Tool: permissions.ToolBash, Command: "printf second"},
	}
	for index, envelope := range []ToolCallEnvelope{first, second} {
		payload, err := json.Marshal(piToolExtensionRequest{
			SchemaVersion: 1, Capability: extension.capability, Envelope: envelope,
		})
		if err != nil {
			t.Fatal(err)
		}
		response := extension.handleCall(context.Background(), payload, int64(index+1))
		if _, result, err := decodePiToolExtensionResponse(response); err != nil ||
			result.Verdict != permissions.VerdictAllow {
			t.Fatalf("call %d response=%s result=%#v err=%v", index+1, response, result, err)
		}
	}
	if len(extension.resolvedCalls) != 2 ||
		extension.resolvedCalls[0].Envelope != first ||
		extension.resolvedCalls[1].Envelope != second {
		t.Fatalf("resolved calls=%#v", extension.resolvedCalls)
	}
	if err := extension.Acknowledge(
		context.Background(), attemptpayload.ProofHarnessFinalOutput,
	); err != nil {
		t.Fatal(err)
	}
	if hook.acked != 2 || !extension.acknowledged ||
		!extension.resolvedCalls[0].Acknowledged ||
		!extension.resolvedCalls[1].Acknowledged {
		t.Fatalf("acked=%d extension=%#v", hook.acked, extension.resolvedCalls)
	}
	if response := extension.handleCall(context.Background(), []byte(`{}`), 5); !bytes.Contains(
		response, []byte(`"status":"denied"`),
	) {
		t.Fatalf("over-budget response=%s", response)
	}
}

func TestPiToolExtensionSourceIsDeterministicAndContainsNoAuthorityOrSecret(t *testing.T) {
	one := piToolExtensionSource("/private/tool.sock", strings.Repeat("1", 32))
	two := piToolExtensionSource("/private/tool.sock", strings.Repeat("1", 32))
	if !reflect.DeepEqual(one, two) ||
		!bytes.Contains(one, []byte(`name: "loom_tool"`)) ||
		!bytes.Contains(one, []byte(`executionMode: "sequential"`)) ||
		!bytes.Contains(one, []byte(`additionalProperties: false`)) ||
		!bytes.Contains(one, []byte(`received.length > 32768`)) ||
		bytes.Contains(one, []byte("API_KEY")) ||
		bytes.Contains(one, []byte("Authorization")) ||
		bytes.Contains(one, []byte("approval-tool-1")) {
		t.Fatalf("tool extension source drifted: %s", one)
	}
}

func TestPiToolExtensionSourceAndPromptPublishOnlyFrozenRemoteTools(t *testing.T) {
	allowed := map[permissions.ToolKind]struct{}{
		permissions.ToolBash:      {},
		permissions.ToolEdit:      {},
		permissions.ToolRead:      {},
		permissions.ToolGrep:      {},
		permissions.ToolWebSearch: {},
	}
	source := piToolExtensionSource(
		"/private/tool.sock", strings.Repeat("1", 32), allowed,
	)
	prompt := toolCallSystemPrompt(allowed, false)
	if !bytes.Contains(source, []byte(`"WebSearch"`)) ||
		!bytes.Contains(source, []byte(`["Grep","Read","WebSearch"].includes`)) ||
		bytes.Contains(source, []byte(`"WebFetch"`)) ||
		bytes.Contains(source, []byte(`"MCPTool"`)) ||
		!strings.Contains(prompt, "WebSearch") ||
		strings.Contains(prompt, "WebFetch") || strings.Contains(prompt, "MCPTool") {
		t.Fatalf("source/prompt capability drift:\n%s\n---\n%s", source, prompt)
	}
}

func TestPiToolExtensionDeniedResultNeedsNoDeliveryAcknowledgement(t *testing.T) {
	extension := &piToolExtension{
		resolved: true,
		result: ToolCallResult{
			Verdict: permissions.VerdictDeny, DenialReason: "denied",
		},
	}
	if err := extension.Acknowledge(
		context.Background(), attemptpayload.ProofHarnessFinalOutput,
	); err != nil {
		t.Fatal(err)
	}
}

func TestPiToolExtensionPendingAskStopsOnAttemptCancellation(t *testing.T) {
	hook := &piToolPendingHookFixture{called: make(chan struct{})}
	binding := ToolCallBinding{
		ConversationID: "conversation-tool-1", WorkItemID: "work-tool-1",
		RunID: "run-tool-1", ClaimGeneration: 1,
		RuntimeInstanceID: "runtime-tool-1", AgentInstanceID: "agent-tool-1",
		ExecutionBindingDigest: strings.Repeat("c", 64),
		CapsuleDigest:          strings.Repeat("d", 64), ClaimID: "claim-tool-1",
		IncidentID: "incident-tool-1", JourneyID: "incident-tool-1",
	}
	extension := &piToolExtension{
		capability: strings.Repeat("1", 32), hook: hook, binding: binding,
	}
	envelope := ToolCallEnvelope{
		JobID: "work-tool-1",
		Call: permissions.ProposedCall{
			Tool: permissions.ToolBash, Command: "printf private-tool-command",
		},
	}
	payload, err := json.Marshal(piToolExtensionRequest{
		SchemaVersion: 1, Capability: extension.capability, Envelope: envelope,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan []byte, 1)
	go func() { done <- extension.handle(ctx, payload) }()
	<-hook.called
	cancel()
	response := <-done
	if !bytes.Contains(response, []byte(`"status":"denied"`)) ||
		bytes.Contains(response, []byte("private-tool-command")) || extension.resolved {
		t.Fatalf("cancelled Ask response=%s resolved=%t", response, extension.resolved)
	}
}

func TestPiToolExtensionReturnsBoundReadContentWithDigestOnlyAuthority(t *testing.T) {
	content := []byte("private source content\n")
	digest := "sha256:" + piContextDigest(content)
	hook := &piToolReadHookFixture{content: content}
	hook.result = ToolCallResult{
		Verdict: permissions.VerdictAllow, ExecutionID: "execution-read-1",
		OutputDigest: digest, ContentDigest: digest,
		Delivery: &ToolCallDelivery{
			Binding: attemptpayload.Binding{
				PayloadID: "payload-read-1",
				Scope: attemptpayload.Scope{
					ConversationID: "conversation-tool-1", WorkItemID: "work-tool-1",
					RunID: "run-tool-1", ClaimGeneration: 1,
					RuntimeInstanceID:      "runtime-tool-1",
					ExecutionBindingDigest: strings.Repeat("c", 64),
					CapsuleDigest:          strings.Repeat("d", 64),
				},
				CallID: "call-read-1", Sequence: 1,
				ContentType:   "text/plain; charset=utf-8",
				ContentDigest: strings.TrimPrefix(digest, "sha256:"),
			},
		},
	}
	binding := ToolCallBinding{
		ConversationID: "conversation-tool-1", WorkItemID: "work-tool-1",
		RunID: "run-tool-1", ClaimGeneration: 1,
		RuntimeInstanceID: "runtime-tool-1", AgentInstanceID: "agent-tool-1",
		ExecutionBindingDigest: strings.Repeat("c", 64),
		CapsuleDigest:          strings.Repeat("d", 64), ClaimID: "claim-tool-1",
		IncidentID: "incident-tool-1", JourneyID: "incident-tool-1",
	}
	extension := &piToolExtension{
		capability: strings.Repeat("1", 32), hook: hook, binding: binding,
	}
	envelope := ToolCallEnvelope{
		JobID: "work-tool-1",
		Call:  permissions.ProposedCall{Tool: permissions.ToolRead, Path: "src/main.go"},
	}
	payload, err := json.Marshal(piToolExtensionRequest{
		SchemaVersion: 1, Capability: extension.capability, Envelope: envelope,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := extension.handle(context.Background(), payload)
	if !bytes.Contains(response, []byte("private source content")) ||
		bytes.Contains(response, []byte("src/main.go")) {
		t.Fatalf("read response=%s", response)
	}
	parsedEnvelope, result, err := decodePiToolExtensionResponse(response)
	if err != nil || parsedEnvelope.Call.Tool != permissions.ToolRead ||
		result.ContentDigest != digest || result.OutputDigest != digest {
		t.Fatalf("decoded envelope=%#v result=%#v err=%v", parsedEnvelope, result, err)
	}

	tampered := bytes.Replace(response, []byte("private source content"), []byte("tampered source content"), 1)
	if _, _, err := decodePiToolExtensionResponse(tampered); err == nil {
		t.Fatal("substituted Read content was accepted")
	}
}

func TestPiToolExtensionRejectsSubstitutedRemoteContent(t *testing.T) {
	content := []byte("private bounded WebSearch result\n")
	digest := "sha256:" + piContextDigest(content)
	hook := &piToolReadHookFixture{content: content}
	hook.result = ToolCallResult{
		Verdict: permissions.VerdictAllow, ExecutionID: "execution-web-search-1",
		OutputDigest: digest, ContentDigest: digest,
		Delivery: &ToolCallDelivery{Binding: attemptpayload.Binding{
			PayloadID: "payload-web-search-1",
			Scope: attemptpayload.Scope{
				ConversationID: "conversation-tool-1", WorkItemID: "work-tool-1",
				RunID: "run-tool-1", ClaimGeneration: 1,
				RuntimeInstanceID:      "runtime-tool-1",
				ExecutionBindingDigest: strings.Repeat("c", 64),
				CapsuleDigest:          strings.Repeat("d", 64),
			},
			CallID: "call-web-search-1", Sequence: 1,
			ContentType:   "text/plain; charset=utf-8",
			ContentDigest: strings.TrimPrefix(digest, "sha256:"),
		}},
	}
	extension := &piToolExtension{
		capability: strings.Repeat("1", 32), hook: hook,
		allowedTools: map[permissions.ToolKind]struct{}{
			permissions.ToolWebSearch: {},
		},
		binding: ToolCallBinding{
			ConversationID: "conversation-tool-1", WorkItemID: "work-tool-1",
			RunID: "run-tool-1", ClaimGeneration: 1,
			RuntimeInstanceID: "runtime-tool-1", AgentInstanceID: "agent-tool-1",
			ExecutionBindingDigest: strings.Repeat("c", 64),
			CapsuleDigest:          strings.Repeat("d", 64), ClaimID: "claim-tool-1",
			IncidentID: "incident-tool-1", JourneyID: "incident-tool-1",
		},
	}
	envelope := ToolCallEnvelope{
		JobID: "work-tool-1",
		Call: permissions.ProposedCall{
			Tool: permissions.ToolWebSearch, Path: "Loom governance status",
		},
	}
	payload, err := json.Marshal(piToolExtensionRequest{
		SchemaVersion: 1, Capability: extension.capability, Envelope: envelope,
	})
	if err != nil {
		t.Fatal(err)
	}
	response := extension.handle(context.Background(), payload)
	if !bytes.Contains(response, []byte("private bounded WebSearch result")) {
		t.Fatalf("remote response=%s", response)
	}
	parsed, result, err := decodePiToolExtensionResponse(response)
	if err != nil || parsed.Call.Tool != permissions.ToolWebSearch ||
		result.ContentDigest != digest {
		t.Fatalf("decoded=%#v result=%#v err=%v", parsed, result, err)
	}
	tampered := bytes.Replace(
		response,
		[]byte("private bounded WebSearch result"),
		[]byte("substituted WebSearch response"),
		1,
	)
	if _, _, err := decodePiToolExtensionResponse(tampered); err == nil {
		t.Fatal("substituted WebSearch content was accepted")
	}
}
