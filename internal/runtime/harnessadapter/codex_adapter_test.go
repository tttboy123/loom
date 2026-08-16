package harnessadapter

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/attemptpayload"
	"loom-pi-rebuild/internal/contextcapsule"
	"loom-pi-rebuild/internal/permissions"
	loomruntime "loom-pi-rebuild/internal/runtime"
	"loom-pi-rebuild/internal/supervisor"
	"loom-pi-rebuild/internal/work"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

func TestCodexAdapterConsumesExactFrozenOpenAIAccount(t *testing.T) {
	access := &credentialAccessFixture{secret: []byte("private-openai-key")}
	runner := &processRunnerFixture{result: HarnessProcessResult{
		Content: "Implemented with Codex",
		Accounting: &work.RunAccounting{
			UsageObserved: true, InputTokens: 140, OutputTokens: 32,
			TotalTokens: 172, CacheReadTokens: 25,
		},
	}}
	diagnostics := &diagnosticRecorderFixture{}
	adapter, err := NewCodexAdapter(CodexAdapterConfig{
		RuntimeInstanceID: "runtime.codex.local", ExecutablePath: "/opt/loom/bin/codex",
		CredentialAccess: access, Diagnostics: diagnostics, Runner: runner,
		Now:            func() time.Time { return time.Date(2026, 8, 10, 16, 0, 0, 0, time.UTC) },
		MaxOutputBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := codexAdapterRequest(t, CodexProviderID, CodexModelID)
	result, err := adapter.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if access.uses != 1 || !reflect.DeepEqual(access.binding, request.ExecutionBinding) ||
		runner.runs != 1 || string(runner.secret) != "private-openai-key" {
		t.Fatalf("access=%#v runner=%#v", access, runner)
	}
	if !strings.Contains(runner.request.SystemPrompt, "provider=openai") ||
		!strings.Contains(runner.request.SystemPrompt, "model="+CodexModelID) ||
		!strings.Contains(runner.request.SystemPrompt, "MCPTool") ||
		strings.Contains(runner.request.SystemPrompt, "Bash") ||
		strings.Contains(runner.request.SystemPrompt, "Edit") ||
		strings.Contains(runner.request.SystemPrompt, "WebSearch") {
		t.Fatalf("system prompt = %q", runner.request.SystemPrompt)
	}
	accounting, ok := result.Accounting()
	if !ok || accounting.TotalTokens != 172 || accounting.CacheReadTokens != 25 {
		t.Fatalf("accounting = %#v, %t", accounting, ok)
	}
	frames := request.FrameSink.(*frameSinkFixture).frames
	if len(frames) != 3 ||
		string(frames[1].Payload()) != `{"delta":"Implemented with Codex"}` ||
		string(frames[2].Payload()) != `{"status":"succeeded","reason":""}` {
		t.Fatalf("frames = %#v", frames)
	}
	if len(diagnostics.records) != 1 ||
		diagnostics.records[0].ProviderID != CodexProviderID ||
		diagnostics.records[0].ProviderAccountID != "openai.primary" ||
		diagnostics.records[0].ModelID != CodexModelID {
		t.Fatalf("diagnostics = %#v", diagnostics.records)
	}
}

func TestCodexAdapterUsesVersionLockedPersistentContinuationForAgentInputs(t *testing.T) {
	access := &credentialAccessFixture{secret: []byte("private-openai-key")}
	inputs := &codexNoAgentInputSourceFixture{}
	runner := &codexAdapterContinuationRunnerFixture{result: HarnessProcessResult{
		Content: "continued in one Codex thread",
	}}
	adapter, err := NewCodexAdapter(CodexAdapterConfig{
		RuntimeInstanceID: "runtime.codex.local",
		ExecutablePath:    "/opt/loom/bin/codex",
		CredentialAccess:  access,
		Diagnostics:       &diagnosticRecorderFixture{},
		Runner:            runner,
		Now:               func() time.Time { return time.Now().UTC() },
		MaxOutputBytes:    64 << 10,
		ContinuationConformance: func(adapterType, path string) bool {
			return adapterType == CodexAdapterType && path == "/opt/loom/bin/codex"
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	consumer, ok := adapter.(loomruntime.AgentInputConsumer)
	if !ok || !consumer.AcceptsAgentInputs() {
		t.Fatal("version-locked Codex Adapter did not advertise Agent Input support")
	}
	request := codexAdapterRequest(t, CodexProviderID, CodexModelID)
	request.AgentInputs = inputs
	if _, err := adapter.Execute(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if runner.oneShotRuns != 0 || runner.continuationRuns != 1 ||
		runner.inputs != inputs || access.uses != 1 {
		t.Fatalf("runner=%#v access=%d", runner, access.uses)
	}
}

func TestCodexAdapterRejectsAgentInputsWithoutContinuationConformance(t *testing.T) {
	access := &credentialAccessFixture{secret: []byte("must-not-be-read")}
	runner := &codexAdapterContinuationRunnerFixture{}
	adapter, err := NewCodexAdapter(CodexAdapterConfig{
		RuntimeInstanceID:       "runtime.codex.local",
		ExecutablePath:          "/opt/loom/bin/codex",
		CredentialAccess:        access,
		Diagnostics:             &diagnosticRecorderFixture{},
		Runner:                  runner,
		Now:                     func() time.Time { return time.Now().UTC() },
		MaxOutputBytes:          64 << 10,
		ContinuationConformance: func(string, string) bool { return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	consumer, ok := adapter.(loomruntime.AgentInputConsumer)
	if !ok || consumer.AcceptsAgentInputs() {
		t.Fatal("unverified Codex executable advertised Agent Input support")
	}
	request := codexAdapterRequest(t, CodexProviderID, CodexModelID)
	request.AgentInputs = &codexNoAgentInputSourceFixture{}
	if _, err := adapter.Execute(context.Background(), request); !errors.Is(err, ErrHarnessExecutionBindingChanged) {
		t.Fatalf("Execute() error = %v", err)
	}
	if access.uses != 0 || runner.continuationRuns != 0 || runner.oneShotRuns != 0 {
		t.Fatalf("drift reached credential or process: access=%d runner=%#v", access.uses, runner)
	}
}

type codexAdapterContinuationRunnerFixture struct {
	result           HarnessProcessResult
	err              error
	oneShotRuns      int
	continuationRuns int
	inputs           loomruntime.AgentInputSource
}

func (runner *codexAdapterContinuationRunnerFixture) RunHarness(
	context.Context,
	HarnessProcessRequest,
	[]byte,
) (HarnessProcessResult, error) {
	runner.oneShotRuns++
	return runner.result, runner.err
}

func (*codexAdapterContinuationRunnerFixture) SupportsAgentInputs() bool { return true }

func (runner *codexAdapterContinuationRunnerFixture) RunHarnessWithAgentInputs(
	_ context.Context,
	_ HarnessProcessRequest,
	_ []byte,
	inputs loomruntime.AgentInputSource,
) (HarnessProcessResult, error) {
	runner.continuationRuns++
	runner.inputs = inputs
	return runner.result, runner.err
}

func TestCodexAdapterInjectsAttemptScopedContextMCPAfterAuthorityValidation(t *testing.T) {
	request := codexAdapterRequest(t, CodexProviderID, CodexModelID)
	request = harnessContextAdapterRequest(t, request, "context:codex:v1")
	access := &credentialAccessFixture{secret: []byte("private-openai-key")}
	runner := &processRunnerFixture{result: HarnessProcessResult{
		Content: "done", Accounting: &work.RunAccounting{
			UsageObserved: true, InputTokens: 1, OutputTokens: 1, TotalTokens: 2,
		},
	}}
	adapter, err := NewCodexAdapter(CodexAdapterConfig{
		RuntimeInstanceID: request.Binding.RuntimeInstanceID,
		ExecutablePath:    "/opt/loom/bin/codex", CredentialAccess: access,
		Diagnostics: &diagnosticRecorderFixture{}, Runner: runner,
		Now: func() time.Time { return time.Now().UTC() }, MaxOutputBytes: 4096,
		ContextConformance: func(adapterType, path string) bool {
			return adapterType == CodexAdapterType && path == "/opt/loom/bin/codex"
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Execute(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if access.uses != 1 || runner.runs != 1 ||
		!validHarnessContextMCPLease(runner.request.ContextMCP) ||
		!strings.Contains(runner.request.SystemPrompt, "MCP") {
		t.Fatalf("access=%d runner=%#v", access.uses, runner)
	}
}

func TestCodexAdapterCancellationRevokesContextMCPBeforeRunnerReturns(t *testing.T) {
	request := harnessContextAdapterRequest(
		t, codexAdapterRequest(t, CodexProviderID, CodexModelID), "context:codex:v1",
	)
	leaseReady := make(chan HarnessContextMCPLease, 1)
	release := make(chan struct{})
	runner := &processRunnerFixture{
		result: HarnessProcessResult{Content: "must not complete"},
		hook: func(_ context.Context, processRequest HarnessProcessRequest) error {
			leaseReady <- processRequest.ContextMCP
			<-release
			return nil
		},
	}
	adapter, err := NewCodexAdapter(CodexAdapterConfig{
		RuntimeInstanceID: request.Binding.RuntimeInstanceID,
		ExecutablePath:    "/opt/loom/bin/codex",
		CredentialAccess:  &credentialAccessFixture{secret: []byte("private-openai-key")},
		Diagnostics:       &diagnosticRecorderFixture{}, Runner: runner,
		Now: func() time.Time { return time.Now().UTC() }, MaxOutputBytes: 4096,
		ContextConformance: func(string, string) bool { return true },
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	finished := make(chan error, 1)
	go func() {
		_, executeErr := adapter.Execute(ctx, request)
		finished <- executeErr
	}()
	lease := <-leaseReady
	address := strings.TrimSuffix(
		strings.TrimPrefix(lease.URL, "http://"), "/mcp",
	)
	cancel()
	deadline := time.Now().Add(time.Second)
	for {
		connection, dialErr := net.DialTimeout("tcp4", address, 50*time.Millisecond)
		if connection != nil {
			_ = connection.Close()
		}
		if dialErr != nil {
			break
		}
		if time.Now().After(deadline) {
			close(release)
			t.Fatal("production Context MCP survived Attempt cancellation")
		}
		time.Sleep(10 * time.Millisecond)
	}
	close(release)
	select {
	case err := <-finished:
		if err == nil {
			t.Fatal("cancelled Codex Adapter returned success")
		}
	case <-time.After(time.Second):
		t.Fatal("Codex Adapter did not return after runner release")
	}
}

func TestCodexAdapterAcknowledgesContextOnlyAfterValidatedHarnessFinalOutput(t *testing.T) {
	request := harnessContextAdapterRequest(
		t, codexAdapterRequest(t, CodexProviderID, CodexModelID), "context:codex:v1",
	)
	delivery := request.ContextDelivery.(*harnessContextDeliveryFixture)
	runner := &processRunnerFixture{
		result: HarnessProcessResult{Content: "validated final output"},
		hook: func(ctx context.Context, processRequest HarnessProcessRequest) error {
			payload, err := json.Marshal(map[string]any{
				"jsonrpc": "2.0", "id": "context", "method": "tools/call",
				"params": map[string]any{
					"name": "loom_read_context",
					"arguments": map[string]any{
						"item_id":        "omitted",
						"content_digest": harnessContextDigest([]byte("retrieved")),
					},
				},
			})
			if err != nil {
				return err
			}
			httpRequest, err := http.NewRequestWithContext(
				ctx, http.MethodPost, processRequest.ContextMCP.URL, bytes.NewReader(payload),
			)
			if err != nil {
				return err
			}
			httpRequest.Header.Set("Content-Type", "application/json")
			httpRequest.Header.Set("Authorization", "Bearer "+processRequest.ContextMCP.Token)
			response, err := http.DefaultClient.Do(httpRequest)
			if err != nil {
				return err
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				return err
			}
			if response.StatusCode != http.StatusOK || !bytes.Contains(body, []byte("retrieved")) {
				return fmt.Errorf("context MCP status=%d body=%s", response.StatusCode, body)
			}
			if delivery.acks != 0 || delivery.payload.Status != attemptpayload.StatusPending {
				return errors.New("HTTP write prematurely acknowledged context delivery")
			}
			return nil
		},
	}
	adapter, err := NewCodexAdapter(CodexAdapterConfig{
		RuntimeInstanceID: request.Binding.RuntimeInstanceID,
		ExecutablePath:    "/opt/loom/bin/codex",
		CredentialAccess:  &credentialAccessFixture{secret: []byte("private-openai-key")},
		Diagnostics:       &diagnosticRecorderFixture{}, Runner: runner,
		Now: func() time.Time { return time.Now().UTC() }, MaxOutputBytes: 4096,
		ContextConformance: func(string, string) bool { return true },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Execute(context.Background(), request); err != nil {
		t.Fatal(err)
	}
	if delivery.prepares != 1 || delivery.acks != 1 ||
		delivery.proof != attemptpayload.ProofHarnessFinalOutput ||
		delivery.payload.Status != attemptpayload.StatusDelivered {
		t.Fatalf("Harness delivery = %#v", delivery)
	}
}

func TestHarnessAdaptersAcknowledgeMultipleContextReadsOnlyAfterFinalOutput(t *testing.T) {
	tests := []struct {
		name             string
		contextAdapterID string
		executablePath   string
		request          func(testing.TB) supervisor.AdapterRequest
		newAdapter       func(supervisor.AdapterRequest, HarnessProcessRunner) (supervisor.RuntimeAdapter, error)
	}{
		{
			name: "Codex", contextAdapterID: "context:codex:v1",
			executablePath: "/opt/loom/bin/codex",
			request: func(t testing.TB) supervisor.AdapterRequest {
				return codexAdapterRequest(t, CodexProviderID, CodexModelID)
			},
			newAdapter: func(request supervisor.AdapterRequest, runner HarnessProcessRunner) (supervisor.RuntimeAdapter, error) {
				return NewCodexAdapter(CodexAdapterConfig{
					RuntimeInstanceID: request.Binding.RuntimeInstanceID,
					ExecutablePath:    "/opt/loom/bin/codex",
					CredentialAccess:  &credentialAccessFixture{secret: []byte("private-openai-key")},
					Diagnostics:       &diagnosticRecorderFixture{}, Runner: runner,
					Now: func() time.Time { return time.Now().UTC() }, MaxOutputBytes: 4096,
					ContextConformance: func(string, string) bool { return true },
				})
			},
		},
		{
			name: "Claude Code", contextAdapterID: "context:claude-code:v1",
			executablePath: "/opt/loom/bin/claude",
			request: func(t testing.TB) supervisor.AdapterRequest {
				return claudeCodeAdapterRequest(t, ClaudeCodeProviderID, ClaudeCodeModelID)
			},
			newAdapter: func(request supervisor.AdapterRequest, runner HarnessProcessRunner) (supervisor.RuntimeAdapter, error) {
				return NewClaudeCodeAdapter(ClaudeCodeAdapterConfig{
					RuntimeInstanceID: request.Binding.RuntimeInstanceID,
					ExecutablePath:    "/opt/loom/bin/claude",
					CredentialAccess:  &credentialAccessFixture{secret: []byte("private-anthropic-key")},
					Diagnostics:       &diagnosticRecorderFixture{}, Runner: runner,
					Now: func() time.Time { return time.Now().UTC() }, MaxOutputBytes: 4096,
					ContextConformance: func(string, string) bool { return true },
				})
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := harnessContextAdapterRequest(t, test.request(t), test.contextAdapterID)
			delivery := &harnessMultiContextDeliveryFixture{
				items: make(map[contextcapsule.RetrievalProposal]contextcapsule.RetrievedItem),
			}
			proposals := make([]contextcapsule.RetrievalProposal, 2)
			for index := range proposals {
				content := []byte(fmt.Sprintf("%s governed context %d", test.name, index+1))
				proposal := contextcapsule.RetrievalProposal{
					ItemID:        fmt.Sprintf("omitted-%d", index+1),
					ContentDigest: harnessContextDigest(content),
				}
				proposals[index] = proposal
				delivery.items[proposal] = contextcapsule.RetrievedItem{
					ItemID: proposal.ItemID, Kind: contextcapsule.KindCurrentTaskState,
					Trust: contextcapsule.TrustObserved, Scope: contextcapsule.ScopeTeamShared,
					ContentDigest: proposal.ContentDigest, SourceType: contextcapsule.SourceObservation,
					SourceRef: "observation:" + proposal.ItemID, Content: content,
				}
			}
			request.ContextDelivery = delivery
			runner := &processRunnerFixture{
				result: HarnessProcessResult{Content: "validated final output"},
				hook: func(ctx context.Context, processRequest HarnessProcessRequest) error {
					for index, proposal := range proposals {
						payload, err := json.Marshal(map[string]any{
							"jsonrpc": "2.0", "id": index + 1, "method": "tools/call",
							"params": map[string]any{
								"name": "loom_read_context",
								"arguments": map[string]any{
									"item_id": proposal.ItemID, "content_digest": proposal.ContentDigest,
								},
							},
						})
						if err != nil {
							return err
						}
						httpRequest, err := http.NewRequestWithContext(
							ctx, http.MethodPost, processRequest.ContextMCP.URL, bytes.NewReader(payload),
						)
						if err != nil {
							return err
						}
						httpRequest.Header.Set("Content-Type", "application/json")
						httpRequest.Header.Set("Authorization", "Bearer "+processRequest.ContextMCP.Token)
						response, err := http.DefaultClient.Do(httpRequest)
						if err != nil {
							return err
						}
						body, readErr := io.ReadAll(response.Body)
						_ = response.Body.Close()
						if readErr != nil {
							return readErr
						}
						if response.StatusCode != http.StatusOK ||
							bytes.Contains(body, []byte("context_retrieval_denied")) {
							return fmt.Errorf("context MCP status=%d body=%s", response.StatusCode, body)
						}
						if len(delivery.ackSequences) != 0 {
							return errors.New("Context delivery acknowledged before Harness final output")
						}
					}
					return nil
				},
			}
			adapter, err := test.newAdapter(request, runner)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := adapter.Execute(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(delivery.ackSequences, []int64{1, 2}) {
				t.Fatalf("final-output acknowledgements = %v", delivery.ackSequences)
			}
		})
	}
}

func TestHarnessAdaptersRouteGovernedReadAndGrepThroughAttemptMCP(t *testing.T) {
	tests := []struct {
		name, contextAdapterID string
		request                func(testing.TB) supervisor.AdapterRequest
		newAdapter             func(supervisor.AdapterRequest, HarnessProcessRunner, loomruntime.AttemptToolGateway) (supervisor.RuntimeAdapter, error)
	}{
		{
			name: "Codex", contextAdapterID: "context:codex:v1",
			request: func(t testing.TB) supervisor.AdapterRequest {
				return codexAdapterRequest(t, CodexProviderID, CodexModelID)
			},
			newAdapter: func(request supervisor.AdapterRequest, runner HarnessProcessRunner, gateway loomruntime.AttemptToolGateway) (supervisor.RuntimeAdapter, error) {
				return NewCodexAdapter(CodexAdapterConfig{
					RuntimeInstanceID: request.Binding.RuntimeInstanceID,
					ExecutablePath:    "/opt/loom/bin/codex", CredentialAccess: &credentialAccessFixture{secret: []byte("private-openai-key")},
					Diagnostics: &diagnosticRecorderFixture{}, Runner: runner,
					Now: func() time.Time { return time.Now().UTC() }, MaxOutputBytes: 4096,
					ContextConformance: func(string, string) bool { return true },
					ToolGateway:        gateway, ToolConformance: func(string, string) bool { return true },
				})
			},
		},
		{
			name: "Claude Code", contextAdapterID: "context:claude-code:v1",
			request: func(t testing.TB) supervisor.AdapterRequest {
				return claudeCodeAdapterRequest(t, ClaudeCodeProviderID, ClaudeCodeModelID)
			},
			newAdapter: func(request supervisor.AdapterRequest, runner HarnessProcessRunner, gateway loomruntime.AttemptToolGateway) (supervisor.RuntimeAdapter, error) {
				return NewClaudeCodeAdapter(ClaudeCodeAdapterConfig{
					RuntimeInstanceID: request.Binding.RuntimeInstanceID,
					ExecutablePath:    "/opt/loom/bin/claude", CredentialAccess: &credentialAccessFixture{secret: []byte("private-anthropic-key")},
					Diagnostics: &diagnosticRecorderFixture{}, Runner: runner,
					Now: func() time.Time { return time.Now().UTC() }, MaxOutputBytes: 4096,
					ContextConformance: func(string, string) bool { return true },
					ToolGateway:        gateway, ToolConformance: func(string, string) bool { return true },
				})
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := harnessGovernedToolAdapterRequest(t, test.request(t), test.contextAdapterID)
			gateway := &harnessToolGatewayFixture{
				allowed: []permissions.ToolKind{
					permissions.ToolBash, permissions.ToolRead, permissions.ToolGrep,
					permissions.ToolEdit, permissions.ToolWebSearch, permissions.ToolMCPTool,
				},
				contents: map[permissions.ToolKind][]byte{
					permissions.ToolRead: []byte("bounded source content"),
					permissions.ToolGrep: []byte("src/main.go:12: bounded match"),
				},
			}
			runner := &processRunnerFixture{
				result: HarnessProcessResult{Content: "validated final output"},
				hook: func(ctx context.Context, processRequest HarnessProcessRequest) error {
					if !processRequest.ContextMCP.ContextEnabled || !processRequest.ContextMCP.ReadEnabled ||
						!processRequest.ContextMCP.GrepEnabled {
						return fmt.Errorf("MCP capabilities = %#v", processRequest.ContextMCP)
					}
					calls := []map[string]any{
						{"name": "loom_read_file", "arguments": map[string]any{"path": "src/main.go"}},
						{"name": "loom_grep_files", "arguments": map[string]any{"path": "src", "pattern": "bounded"}},
					}
					for index, call := range calls {
						payload, err := json.Marshal(map[string]any{
							"jsonrpc": "2.0", "id": index + 1, "method": "tools/call", "params": call,
						})
						if err != nil {
							return err
						}
						httpRequest, err := http.NewRequestWithContext(
							ctx, http.MethodPost, processRequest.ContextMCP.URL, bytes.NewReader(payload),
						)
						if err != nil {
							return err
						}
						httpRequest.Header.Set("Content-Type", "application/json")
						httpRequest.Header.Set("Authorization", "Bearer "+processRequest.ContextMCP.Token)
						response, err := http.DefaultClient.Do(httpRequest)
						if err != nil {
							return err
						}
						body, readErr := io.ReadAll(response.Body)
						_ = response.Body.Close()
						if readErr != nil || response.StatusCode != http.StatusOK ||
							!bytes.Contains(body, []byte(`"verdict":"allow"`)) {
							return fmt.Errorf("tool MCP status=%d body=%s read=%v", response.StatusCode, body, readErr)
						}
						if len(gateway.ackSequences) != 0 {
							return errors.New("ToolCall result acknowledged before Harness final output")
						}
					}
					return nil
				},
			}
			adapter, err := test.newAdapter(request, runner, gateway)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := adapter.Execute(context.Background(), request); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(gateway.sequences, []int64{1, 2}) ||
				!reflect.DeepEqual(gateway.ackSequences, []int64{1, 2}) || len(gateway.bindings) != 2 {
				t.Fatalf("sequences=%v ack=%v bindings=%#v", gateway.sequences, gateway.ackSequences, gateway.bindings)
			}
			want := loomruntime.ToolCallBinding{
				ConversationID: request.ContextCapsule.ConversationID,
				WorkItemID:     request.Binding.WorkItemID, RunID: request.Binding.RunID,
				ClaimGeneration:        request.Binding.ClaimGeneration,
				RuntimeInstanceID:      request.Binding.RuntimeInstanceID,
				AgentInstanceID:        request.Binding.SenderAgentInstanceID,
				ExecutionBindingDigest: request.ExecutionBinding.BindingDigest,
				CapsuleDigest:          request.ContextCapsule.CapsuleDigest, ClaimID: request.ClaimID,
				IncidentID: request.IncidentID, JourneyID: request.Dispatch.CorrelationID(),
			}
			if gateway.bindings[0] != want || gateway.bindings[1] != want {
				t.Fatalf("ToolCall bindings = %#v, want %#v", gateway.bindings, want)
			}
		})
	}
}

func TestCodexAdapterRejectsGovernedToolConformanceDriftBeforeCredentialAccess(t *testing.T) {
	request := harnessGovernedToolAdapterRequest(
		t, codexAdapterRequest(t, CodexProviderID, CodexModelID), "context:codex:v1",
	)
	access := &credentialAccessFixture{secret: []byte("must-not-be-read")}
	runner := &processRunnerFixture{}
	adapter, err := NewCodexAdapter(CodexAdapterConfig{
		RuntimeInstanceID: request.Binding.RuntimeInstanceID,
		ExecutablePath:    "/opt/loom/bin/codex", CredentialAccess: access,
		Diagnostics: &diagnosticRecorderFixture{}, Runner: runner,
		Now: func() time.Time { return time.Now().UTC() }, MaxOutputBytes: 4096,
		ContextConformance: func(string, string) bool { return true },
		ToolGateway: &harnessToolGatewayFixture{
			allowed: []permissions.ToolKind{permissions.ToolRead},
		},
		ToolConformance: func(string, string) bool { return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Execute(context.Background(), request); !errors.Is(err, ErrHarnessExecutionBindingChanged) {
		t.Fatalf("Execute() error = %v", err)
	}
	if access.uses != 0 || runner.runs != 0 {
		t.Fatalf("drift reached credential=%d process=%d", access.uses, runner.runs)
	}
}

func harnessGovernedToolAdapterRequest(
	t testing.TB,
	request supervisor.AdapterRequest,
	contextAdapterID string,
) supervisor.AdapterRequest {
	t.Helper()
	request = harnessContextAdapterRequest(t, request, contextAdapterID)
	binding := request.ExecutionBinding
	profile := loomruntime.RuntimeProfile{
		ID: binding.ProfileID, AdapterType: binding.HarnessAdapter,
		ProviderID: binding.ProviderID, ProviderAccountID: binding.ProviderAccountID,
		ModelID: binding.ModelID, AuthMode: binding.AuthMode,
		EndpointFingerprint: binding.EndpointFingerprint,
		CredentialReference: binding.CredentialReference,
		CredentialRevision:  binding.CredentialRevision, ReasoningEffort: binding.ReasoningEffort,
		RequiredCapabilities: append(
			append([]string(nil), binding.Capabilities...), loomruntime.CapabilityGovernedToolLoop,
		),
		Timeout: binding.Timeout, Budget: binding.Budget,
	}
	instance := loomruntime.RuntimeInstance{
		ID: binding.RuntimeInstanceID, DeviceID: "device.local",
		AdapterType: binding.HarnessAdapter, DisplayName: "Harness",
		ExecutableVersion: "verified", Status: loomruntime.RuntimeOnline,
		ObservedCapabilities: append([]string(nil), profile.RequiredCapabilities...), Capacity: 2,
	}
	var err error
	request.ExecutionBinding, err = loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	request.RouteSegment, err = contextcapsule.NewRouteSegmentBinding(
		contextcapsule.RouteSegmentBindingInput{
			SegmentID: "segment-tools", ConversationID: request.ContextCapsule.ConversationID,
			TeamID: request.ContextCapsule.TeamID, AgentID: request.ContextCapsule.AgentID,
			RoleID: request.ContextCapsule.RoleID, AttemptNumber: 1,
			CapsuleDigest:          request.ContextCapsule.CapsuleDigest,
			ExecutionBindingDigest: request.ExecutionBinding.BindingDigest,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	request.ClaimID = "claim-tools"
	request.IncidentID = request.Dispatch.CorrelationID()
	return request
}

func TestCodexAdapterRejectsContextExecutableDriftBeforeCredentialAccess(t *testing.T) {
	request := harnessContextAdapterRequest(
		t, codexAdapterRequest(t, CodexProviderID, CodexModelID), "context:codex:v1",
	)
	access := &credentialAccessFixture{secret: []byte("must-not-be-read")}
	runner := &processRunnerFixture{}
	adapter, err := NewCodexAdapter(CodexAdapterConfig{
		RuntimeInstanceID: request.Binding.RuntimeInstanceID,
		ExecutablePath:    "/opt/loom/bin/codex", CredentialAccess: access,
		Diagnostics: &diagnosticRecorderFixture{}, Runner: runner,
		Now: func() time.Time { return time.Now().UTC() }, MaxOutputBytes: 4096,
		ContextConformance: func(string, string) bool { return false },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Execute(context.Background(), request); !errors.Is(err, ErrHarnessExecutionBindingChanged) ||
		!errors.Is(err, ErrHarnessContextMCP) {
		t.Fatalf("Execute() error = %v", err)
	}
	if access.uses != 0 || runner.runs != 0 {
		t.Fatalf("drift reached credential=%d process=%d", access.uses, runner.runs)
	}
}

func TestCodexAdapterRejectsContextCapabilityMismatchBeforeCredentialAccess(t *testing.T) {
	request := codexAdapterRequest(t, CodexProviderID, CodexModelID)
	request.ContextRetriever = &harnessContextRetrieverFixture{}
	access := &credentialAccessFixture{secret: []byte("must-not-be-read")}
	runner := &processRunnerFixture{}
	adapter, err := NewCodexAdapter(CodexAdapterConfig{
		RuntimeInstanceID: request.Binding.RuntimeInstanceID,
		ExecutablePath:    "/opt/loom/bin/codex", CredentialAccess: access,
		Diagnostics: &diagnosticRecorderFixture{}, Runner: runner,
		Now: func() time.Time { return time.Now().UTC() }, MaxOutputBytes: 4096,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Execute(context.Background(), request); !errors.Is(err, ErrHarnessContextMCP) {
		t.Fatalf("Execute() error = %v", err)
	}
	if access.uses != 0 || runner.runs != 0 {
		t.Fatalf("mismatch reached credential=%d process=%d", access.uses, runner.runs)
	}
}

func harnessContextAdapterRequest(
	t testing.TB,
	request supervisor.AdapterRequest,
	contextAdapterID string,
) supervisor.AdapterRequest {
	t.Helper()
	profile := loomruntime.RuntimeProfile{
		ID: "profile.context", AdapterType: request.ExecutionBinding.HarnessAdapter,
		ProviderID:        request.ExecutionBinding.ProviderID,
		ProviderAccountID: request.ExecutionBinding.ProviderAccountID,
		ModelID:           request.ExecutionBinding.ModelID, AuthMode: request.ExecutionBinding.AuthMode,
		EndpointFingerprint: request.ExecutionBinding.EndpointFingerprint,
		CredentialReference: request.ExecutionBinding.CredentialReference,
		CredentialRevision:  request.ExecutionBinding.CredentialRevision,
		ReasoningEffort:     request.ExecutionBinding.ReasoningEffort,
		RequiredCapabilities: append(
			append([]string(nil), request.ExecutionBinding.Capabilities...),
			loomruntime.CapabilityContextRetrieval,
		),
		Timeout: request.ExecutionBinding.Timeout,
	}
	instance := loomruntime.RuntimeInstance{
		ID: request.ExecutionBinding.RuntimeInstanceID, DeviceID: "device.local",
		AdapterType: request.ExecutionBinding.HarnessAdapter, DisplayName: "Harness",
		ExecutableVersion: "verified", Status: loomruntime.RuntimeOnline,
		ObservedCapabilities: append([]string(nil), profile.RequiredCapabilities...), Capacity: 1,
	}
	binding, err := loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	target := contextcapsule.Target{
		ConversationID: "conversation-context", TeamID: "team-context",
		AgentID: request.Binding.SenderAgentInstanceID, RoleID: "role-context",
		ProviderID: binding.ProviderID, ProviderAccountID: binding.ProviderAccountID,
		ModelID: binding.ModelID, AuthMode: string(binding.AuthMode),
		ContextAdapterID: contextAdapterID, DisclosurePolicyID: "policy.context",
		DisclosurePolicyVersion: 1, TokenBudget: 64,
	}
	capsule, err := contextcapsule.BuildRoleContextCapsule(target, []contextcapsule.ItemInput{
		{
			ItemID: "goal", Kind: contextcapsule.KindConversationGoal,
			Trust: contextcapsule.TrustAuthoritative, Scope: contextcapsule.ScopeTeamShared,
			Priority: contextcapsule.PrioritySystem, TokenCount: 4, Required: true,
			Content:    []byte("Use the admitted Context."),
			SourceType: contextcapsule.SourceAuthority, SourceRef: "goal:context",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	payload, err := contextcapsule.RenderDispatchPayload(capsule)
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID: request.Dispatch.MessageID(), CorrelationID: request.Dispatch.CorrelationID(),
		WorkItemID: request.Binding.WorkItemID, RunID: request.Binding.RunID,
		ClaimGeneration:       request.Binding.ClaimGeneration,
		RuntimeInstanceID:     request.Binding.RuntimeInstanceID,
		SenderAgentInstanceID: request.Binding.SenderAgentInstanceID,
		Sequence:              1, Type: bridgev1.MessageDispatch, EmittedAt: request.Dispatch.EmittedAt(),
		Payload: payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	content := []byte("retrieved")
	request.ExecutionBinding = binding
	request.Dispatch = dispatch
	request.ContextCapsule = capsule.AuthorityRecord()
	retriever := &harnessContextRetrieverFixture{
		want: contextcapsule.RetrievalProposal{
			ItemID: "omitted", ContentDigest: harnessContextDigest(content),
		},
		item: contextcapsule.RetrievedItem{
			ItemID: "omitted", Kind: contextcapsule.KindCurrentTaskState,
			Trust: contextcapsule.TrustObserved, Scope: contextcapsule.ScopeTeamShared,
			ContentDigest: harnessContextDigest(content), SourceType: contextcapsule.SourceObservation,
			SourceRef: "observation:context", Content: content,
		},
	}
	request.ContextRetriever = retriever
	request.ContextDelivery = &harnessContextDeliveryFixture{retriever: retriever}
	return request
}

func TestCodexAdapterRejectsProviderDriftBeforeCredentialAccess(t *testing.T) {
	access := &credentialAccessFixture{secret: []byte("must-not-be-read")}
	runner := &processRunnerFixture{}
	adapter, err := NewCodexAdapter(CodexAdapterConfig{
		RuntimeInstanceID: "runtime.codex.local", ExecutablePath: "/opt/loom/bin/codex",
		CredentialAccess: access, Diagnostics: &diagnosticRecorderFixture{}, Runner: runner,
		Now: func() time.Time { return time.Now().UTC() }, MaxOutputBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := codexAdapterRequest(t, "anthropic", CodexModelID)
	if _, err := adapter.Execute(context.Background(), request); !errors.Is(
		err, ErrHarnessExecutionBindingChanged,
	) {
		t.Fatalf("Execute() error = %v", err)
	}
	if access.uses != 0 || runner.runs != 0 {
		t.Fatalf("drift reached credential=%d process=%d", access.uses, runner.runs)
	}
}

func TestCodexAdapterProjectsProviderTimeoutToExactAccount(t *testing.T) {
	access := &credentialAccessFixture{secret: []byte("private-openai-key")}
	runner := &processRunnerFixture{err: ErrHarnessProviderTimeout}
	diagnostics := &diagnosticRecorderFixture{}
	adapter, err := NewCodexAdapter(CodexAdapterConfig{
		RuntimeInstanceID: "runtime.codex.local", ExecutablePath: "/opt/loom/bin/codex",
		CredentialAccess: access, Diagnostics: diagnostics, Runner: runner,
		Now: func() time.Time {
			return time.Date(2026, 8, 12, 8, 0, 0, 0, time.UTC)
		},
		MaxOutputBytes: 64 << 10,
	})
	if err != nil {
		t.Fatal(err)
	}
	request := codexAdapterRequest(t, CodexProviderID, CodexModelID)
	result, err := adapter.Execute(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	frames := request.FrameSink.(*frameSinkFixture).frames
	if len(frames) != 2 ||
		string(frames[1].Payload()) != `{"status":"failed","reason":"timeout"}` ||
		!result.DispatchAcknowledged() || !result.ResultAcknowledged() {
		t.Fatalf("result=%#v frames=%#v", result, frames)
	}
	if len(diagnostics.records) != 1 ||
		diagnostics.records[0].ProviderID != CodexProviderID ||
		diagnostics.records[0].ProviderAccountID != "openai.primary" ||
		diagnostics.records[0].ModelID != CodexModelID ||
		diagnostics.records[0].Stage != "provider_connect" ||
		diagnostics.records[0].ErrorCode != "timeout" ||
		!diagnostics.records[0].Retryable {
		t.Fatalf("diagnostics = %#v", diagnostics.records)
	}
}

func codexAdapterRequest(
	t testing.TB,
	providerID, modelID string,
) supervisor.AdapterRequest {
	t.Helper()
	profile, err := loomruntime.NewRuntimeProfile(loomruntime.RuntimeProfile{
		ID: "profile.codex.openai.primary.r4", AdapterType: CodexAdapterType,
		ProviderID: providerID, ProviderAccountID: "openai.primary", ModelID: modelID,
		AuthMode:            loomruntime.AuthBrokered,
		EndpointFingerprint: CodexEndpointFingerprint,
		CredentialReference: "credential-ref-openai-primary", CredentialRevision: 4,
		ReasoningEffort:      "high",
		RequiredCapabilities: []string{"reasoning_effort", "workspace_edit"},
		Timeout:              2 * time.Minute,
	})
	if err != nil {
		t.Fatal(err)
	}
	instance, err := loomruntime.NewRuntimeInstance(loomruntime.RuntimeInstance{
		ID: "runtime.codex.local", DeviceID: "device.local", AdapterType: CodexAdapterType,
		DisplayName: "Codex", ExecutableVersion: "0.144.1", Status: loomruntime.RuntimeOnline,
		ObservedCapabilities: []string{"reasoning_effort", "workspace_edit"}, Capacity: 2,
	})
	if err != nil {
		t.Fatal(err)
	}
	binding, err := loomruntime.FreezeExecutionBinding(profile, instance)
	if err != nil {
		t.Fatal(err)
	}
	dispatch, err := bridgev1.NewFrame(bridgev1.FrameInput{
		MessageID:     "77777777-7777-4777-8777-777777777777",
		CorrelationID: "88888888-8888-4888-8888-888888888888",
		WorkItemID:    "work-codex-1", RunID: "run-codex-1", ClaimGeneration: 1,
		RuntimeInstanceID: "runtime.codex.local", SenderAgentInstanceID: "agent-codex-1",
		Sequence: 1, Type: bridgev1.MessageDispatch,
		EmittedAt: time.Date(2026, 8, 10, 15, 59, 59, 0, time.UTC),
		Payload:   []byte(`{"schema_version":1,"kind":"pi_rpc_prompt","prompt":"Implement with Codex"}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	return supervisor.AdapterRequest{
		WorkspacePath: root, HomePath: root, TempPath: root,
		Binding: bridgev1.RunStreamBinding{
			WorkItemID: "work-codex-1", RunID: "run-codex-1", ClaimGeneration: 1,
			RuntimeInstanceID: "runtime.codex.local", SenderAgentInstanceID: "agent-codex-1",
		},
		ExecutionBinding: binding, Dispatch: dispatch, FrameSink: &frameSinkFixture{},
	}
}
