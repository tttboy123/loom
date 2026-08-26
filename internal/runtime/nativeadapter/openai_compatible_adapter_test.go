package nativeadapter

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/contextcapsule"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/supervisor"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

func TestMiniMaxAgentResponseStripsInlineHiddenReasoning(t *testing.T) {
	response, err := decodeDeepSeekAgentResponse(
		[]byte(`{"model":"MiniMax-M3","choices":[{"message":{"role":"assistant","content":"<think>private reasoning must not enter Evidence</think>\nVISIBLE-ONLY"}}]}`),
		MiniMaxAgentModelID,
	)
	if err != nil || response.content != "VISIBLE-ONLY" ||
		strings.Contains(response.content, "private reasoning") {
		t.Fatalf("response=%#v error=%v", response, err)
	}
}

func TestMiniMaxAgentResponseRejectsUnterminatedInlineHiddenReasoning(t *testing.T) {
	response, err := decodeDeepSeekAgentResponse(
		[]byte(`{"model":"MiniMax-M3","choices":[{"message":{"role":"assistant","content":"<think>private reasoning must not enter Evidence"}}]}`),
		MiniMaxAgentModelID,
	)
	if err == nil || response.content != "" {
		t.Fatalf("response=%#v error=%v", response, err)
	}
}

func TestLoomNativeOpenAICompatibleAdaptersConsumeExactProviderBinding(t *testing.T) {
	tests := []struct {
		name                string
		providerID          string
		modelID             string
		endpoint            string
		endpointFingerprint string
		runtimeInstanceID   string
		newAdapter          func(OpenAICompatibleAgentAdapterConfig) (supervisor.RuntimeAdapter, error)
		wantTokenField      string
		forbidTokenField    string
	}{
		{
			name: "Kimi", providerID: KimiAgentProviderID,
			modelID: KimiAgentModelID, endpoint: KimiAgentEndpoint,
			endpointFingerprint: KimiAgentEndpointFingerprint,
			runtimeInstanceID:   "runtime.loom-native.kimi",
			newAdapter:          NewKimiAgentAdapter,
			wantTokenField:      `"max_tokens":2048`,
			forbidTokenField:    `"max_completion_tokens"`,
		},
		{
			name: "MiniMax", providerID: MiniMaxAgentProviderID,
			modelID: MiniMaxAgentModelID, endpoint: MiniMaxAgentEndpoint,
			endpointFingerprint: MiniMaxAgentEndpointFingerprint,
			runtimeInstanceID:   "runtime.loom-native.minimax",
			newAdapter:          NewMiniMaxAgentAdapter,
			wantTokenField:      `"max_completion_tokens":2048`,
			forbidTokenField:    `"max_tokens"`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			secretValue := "private-" + strings.ToLower(test.name) + "-key"
			access := &credentialAccessFixture{secret: []byte(secretValue)}
			diagnostics := &agentDiagnosticRecorderFixture{}
			doer := &httpDoerFixture{response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(strings.NewReader(
					`{"model":"` + test.modelID + `","choices":[{"message":{"role":"assistant","content":"Implemented safely"}}],` +
						`"usage":{"prompt_tokens":80,"completion_tokens":21,"total_tokens":101,` +
						`"prompt_tokens_details":{"cached_tokens":10}}}`,
				)),
			}}
			adapter, err := test.newAdapter(OpenAICompatibleAgentAdapterConfig{
				RuntimeInstanceID: test.runtimeInstanceID,
				CredentialAccess:  access,
				Diagnostics:       diagnostics,
				Client:            doer,
				Now: func() time.Time {
					return time.Date(2026, 8, 10, 12, 0, 0, 0, time.UTC)
				},
				MaxResponseBytes: 64 << 10,
			})
			if err != nil {
				t.Fatal(err)
			}
			request := openAICompatibleAdapterRequest(
				t,
				test.runtimeInstanceID,
				test.providerID,
				test.modelID,
				test.endpointFingerprint,
			)
			result, err := adapter.Execute(context.Background(), request)
			if err != nil {
				t.Fatal(err)
			}
			if access.uses != 1 || !reflect.DeepEqual(access.binding, request.ExecutionBinding) {
				t.Fatalf("credential access = uses %d binding %#v", access.uses, access.binding)
			}
			if doer.requests != 1 || doer.request == nil ||
				doer.request.Method != http.MethodPost ||
				doer.request.URL.String() != test.endpoint ||
				doer.auth != "Bearer "+secretValue ||
				doer.request.Header.Get("Authorization") != "" ||
				!bytes.Contains(doer.body, []byte(`"model":"`+test.modelID+`"`)) ||
				!bytes.Contains(doer.body, []byte("provider="+test.providerID)) ||
				!bytes.Contains(doer.body, []byte("model="+test.modelID)) ||
				!bytes.Contains(doer.body, []byte("No tools are available for this Agent attempt")) ||
				bytes.Contains(doer.body, []byte("built on OpenAI")) ||
				!bytes.Contains(doer.body, []byte(test.wantTokenField)) ||
				bytes.Contains(doer.body, []byte(test.forbidTokenField)) ||
				bytes.Contains(doer.body, []byte(secretValue)) {
				t.Fatalf("request = %#v auth=%q body=%s", doer.request, doer.auth, doer.body)
			}
			accounting, ok := result.Accounting()
			if !ok || accounting.InputTokens != 80 || accounting.OutputTokens != 21 ||
				accounting.TotalTokens != 101 || accounting.CacheReadTokens != 10 {
				t.Fatalf("accounting = %#v, %t", accounting, ok)
			}
			if len(diagnostics.records) != 1 ||
				diagnostics.records[0].ProviderID != test.providerID ||
				diagnostics.records[0].ProviderAccountID != test.providerID+".primary" ||
				diagnostics.records[0].ModelID != test.modelID ||
				diagnostics.records[0].Stage != "agent_attempt_dispatch" ||
				diagnostics.records[0].Result != "succeeded" {
				t.Fatalf("diagnostics = %#v", diagnostics.records)
			}
		})
	}
}

func TestLoomNativeKimiAndMiniMaxUseSameScopedContextToolBoundary(t *testing.T) {
	tests := []struct {
		name                string
		providerID          string
		modelID             string
		endpointFingerprint string
		runtimeInstanceID   string
		newAdapter          func(OpenAICompatibleAgentAdapterConfig) (supervisor.RuntimeAdapter, error)
	}{
		{
			name: "Kimi", providerID: KimiAgentProviderID, modelID: KimiAgentModelID,
			endpointFingerprint: KimiAgentEndpointFingerprint,
			runtimeInstanceID:   "runtime.loom-native.kimi", newAdapter: NewKimiAgentAdapter,
		},
		{
			name: "MiniMax", providerID: MiniMaxAgentProviderID, modelID: MiniMaxAgentModelID,
			endpointFingerprint: MiniMaxAgentEndpointFingerprint,
			runtimeInstanceID:   "runtime.loom-native.minimax", newAdapter: NewMiniMaxAgentAdapter,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, omitted := retrievalAdapterRequest(
				t,
				openAICompatibleAdapterRequestWithCapabilities(
					t, test.runtimeInstanceID, test.providerID, test.modelID,
					test.endpointFingerprint,
					[]string{loomruntime.CapabilityContextRetrieval},
				),
			)
			retrievedBody := "scoped omitted context must only enter the second Provider request"
			retriever := &contextRetrieverFixture{
				want: contextcapsule.RetrievalProposal{
					ItemID: omitted.ItemID, ContentDigest: omitted.ContentDigest,
					ArtifactRef: omitted.ArtifactRef,
				},
				content: []byte(retrievedBody),
			}
			request.ContextRetriever = retriever
			request.ContextDelivery = &contextDeliveryFixture{retriever: retriever}
			doer := &contextRetrievalHTTPDoerFixture{responses: []*http.Response{
				{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(strings.NewReader(
						`{"model":"` + test.modelID + `","choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"call-context-1","type":"function","function":{"name":"loom_read_context","arguments":"{\"item_id\":\"` + omitted.ItemID + `\",\"content_digest\":\"` + omitted.ContentDigest + `\",\"artifact_ref\":\"` + omitted.ArtifactRef + `\"}"}}]}}]}`,
					)),
				},
				{
					StatusCode: http.StatusOK,
					Body: io.NopCloser(strings.NewReader(
						`{"model":"` + test.modelID + `","choices":[{"message":{"role":"assistant","content":"final scoped answer"}}]}`,
					)),
				},
			}}
			adapter, err := test.newAdapter(OpenAICompatibleAgentAdapterConfig{
				RuntimeInstanceID: test.runtimeInstanceID,
				CredentialAccess:  &credentialAccessFixture{secret: []byte("private-provider-key")},
				Diagnostics:       &agentDiagnosticRecorderFixture{}, Client: doer,
				Now:              func() time.Time { return time.Date(2026, 8, 12, 14, 0, 0, 0, time.UTC) },
				MaxResponseBytes: 64 << 10,
			})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := adapter.Execute(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			if len(doer.bodies) != 2 || bytes.Contains(doer.bodies[0], []byte(retrievedBody)) ||
				!bytes.Contains(doer.bodies[1], []byte(retrievedBody)) ||
				!bytes.Contains(doer.bodies[0], []byte(`"name":"loom_read_context"`)) {
				t.Fatalf("Provider rounds = %#v", doer.bodies)
			}
		})
	}
}

func TestLoomNativeProviderAdaptersRejectCrossProviderBindingBeforeCredentialAccess(t *testing.T) {
	access := &credentialAccessFixture{secret: []byte("must-not-be-read")}
	doer := &httpDoerFixture{}
	diagnostics := &agentDiagnosticRecorderFixture{}
	adapter, err := NewKimiAgentAdapter(OpenAICompatibleAgentAdapterConfig{
		RuntimeInstanceID: "runtime.loom-native.kimi",
		CredentialAccess:  access,
		Diagnostics:       diagnostics,
		Client:            doer,
		Now:               func() time.Time { return time.Now().UTC() },
		MaxResponseBytes:  64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := openAICompatibleAdapterRequest(
		t,
		"runtime.loom-native.kimi",
		MiniMaxAgentProviderID,
		MiniMaxAgentModelID,
		MiniMaxAgentEndpointFingerprint,
	)
	if _, err := adapter.Execute(context.Background(), request); !errors.Is(
		err,
		ErrAgentExecutionBindingChanged,
	) {
		t.Fatalf("Execute() error = %v", err)
	}
	if access.uses != 0 || doer.requests != 0 {
		t.Fatalf("cross-provider binding used credential=%d requests=%d", access.uses, doer.requests)
	}
	if len(diagnostics.records) != 1 ||
		diagnostics.records[0].Stage != "agent_attempt_dispatch" ||
		diagnostics.records[0].ErrorCode != "binding_changed" {
		t.Fatalf("diagnostics = %#v", diagnostics.records)
	}
}

func openAICompatibleAdapterRequest(
	t testing.TB,
	runtimeInstanceID string,
	providerID string,
	modelID string,
	endpointFingerprint string,
) supervisor.AdapterRequest {
	t.Helper()
	return openAICompatibleAdapterRequestWithCapabilities(
		t, runtimeInstanceID, providerID, modelID, endpointFingerprint, nil,
	)
}

func openAICompatibleAdapterRequestWithCapabilities(
	t testing.TB,
	runtimeInstanceID string,
	providerID string,
	modelID string,
	endpointFingerprint string,
	capabilities []string,
) supervisor.AdapterRequest {
	t.Helper()
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID:                   providerID + "-agent-primary-v1",
		AdapterType:          DeepSeekAgentAdapterType,
		ProviderID:           providerID,
		ProviderAccountID:    providerID + ".primary",
		ModelID:              modelID,
		AuthMode:             loomruntime.AuthBrokered,
		EndpointFingerprint:  endpointFingerprint,
		CredentialReference:  "credential-ref-" + providerID + "-primary",
		CredentialRevision:   3,
		RequiredCapabilities: append([]string(nil), capabilities...),
		Timeout:              30 * time.Second,
		Budget:               int64Pointer(2500),
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: runtimeInstanceID, DeviceID: "device-local",
		AdapterType: DeepSeekAgentAdapterType, DisplayName: "Loom Native",
		Status:               loomruntime.RuntimeOnline,
		ObservedCapabilities: append([]string(nil), capabilities...),
		Capacity:             3,
	})
	if err != nil {
		t.Fatal(err)
	}
	binding, err := loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:             "33333333-3333-4333-8333-333333333333",
		CorrelationID:         "44444444-4444-4444-8444-444444444444",
		WorkItemID:            "work-provider-1",
		RunID:                 "run-provider-1",
		ClaimGeneration:       1,
		RuntimeInstanceID:     runtimeInstanceID,
		SenderAgentInstanceID: "agent-provider-1",
		Sequence:              1,
		Type:                  bridgev1.MessageDispatch,
		EmittedAt:             time.Date(2026, 8, 10, 11, 59, 59, 0, time.UTC),
		Payload:               []byte(`{"schema_version":1,"kind":"pi_rpc_prompt","prompt":"Implement the bounded change"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	return supervisor.AdapterRequest{
		Binding: bridgev1.RunStreamBinding{
			WorkItemID: "work-provider-1", RunID: "run-provider-1",
			ClaimGeneration: 1, RuntimeInstanceID: runtimeInstanceID,
			SenderAgentInstanceID: "agent-provider-1",
		},
		ExecutionBinding: binding,
		Dispatch:         dispatch,
		FrameSink:        &frameSinkFixture{},
	}
}
