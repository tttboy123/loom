package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/harnessgateway"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/provider"
	"loom-pi-rebuild/internal/runtime/piadapter"
	"loom-pi-rebuild/internal/state"
)

func TestCOMP2CConversationRouteHandoffBindsAndRevokesChat(t *testing.T) {
	slot := &productConversationRouteSlot{}
	constructed := 0
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(
			context.Context, localipc.Request,
		) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"44444444-4444-4444-8444-444444444444",
		nil,
		productCompatibilityConstruction{
			conversationSlot: slot,
			conversationFactory: func(
				context.Context,
			) (productConversationRoutes, error) {
				constructed++
				return productConversationRoutes{
					route: productConversationRouteFixture{},
					close: func() error { return nil },
				}, nil
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if constructed != 1 || !slot.Ready() {
		t.Fatalf("constructed=%d ready=%t", constructed, slot.Ready())
	}
	thread, err := slot.ChatThread(context.Background(), "thread-route")
	if err != nil || thread.ThreadID != "thread-route" {
		t.Fatalf("thread=%#v err=%v", thread, err)
	}
	thread, err = slot.SendMessage(
		context.Background(),
		api.LocalProductChatMessageRequest{
			ThreadID: "thread-route", Content: "bounded fixture",
		},
	)
	if err != nil || len(thread.Messages) != 1 ||
		thread.Messages[0].Content != "bounded fixture" {
		t.Fatalf("sent thread=%#v err=%v", thread, err)
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	if slot.Ready() {
		t.Fatal("Conversation slot remained ready after Composition close")
	}
	if _, err := slot.ChatThread(
		context.Background(), "thread-route",
	); !errors.Is(err, api.ErrLocalProductChatUnavailable) {
		t.Fatalf("closed Conversation route err=%v", err)
	}
}

func TestCOMP2CConversationFailurePreventsAgentRuntimeActivation(t *testing.T) {
	conversationSlot := &productConversationRouteSlot{}
	agentStarted := false
	want := errors.New("Conversation construction failed")
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(
			context.Context, localipc.Request,
		) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"55555555-5555-4555-8555-555555555555",
		nil,
		productCompatibilityConstruction{
			conversationSlot: conversationSlot,
			conversationFactory: func(
				context.Context,
			) (productConversationRoutes, error) {
				return productConversationRoutes{}, want
			},
			assetSlot: &productAssetRouteSlot{},
			assetFactory: func(context.Context) (productAssetBundle, error) {
				return productAssetBundleFixture(nil), nil
			},
			workSlot: &productWorkRouteSlot{},
			workFactory: func(context.Context) (productWorkRoutes, error) {
				return productWorkRoutesFixture(), nil
			},
			agentRuntimeSlot: &productAgentRuntimeRouteSlot{},
			agentRuntimeFactory: func(
				context.Context,
			) (productAgentRuntimeRoutes, error) {
				agentStarted = true
				return productAgentRuntimeRoutes{
					mission:      productMissionExecutionRouteFixture{},
					handoff:      productHandoffRouteFixture{},
					roundtable:   productRoundtableRouteFixture{},
					materializer: productSavedTeamMaterializerFixture{},
					close:        func() error { return nil },
				}, nil
			},
		},
	)
	if facade != nil || !errors.Is(err, want) || conversationSlot.Ready() || agentStarted {
		t.Fatalf(
			"facade=%#v err=%v conversationReady=%t agentStarted=%t",
			facade, err, conversationSlot.Ready(), agentStarted,
		)
	}
	if got := productCompatibilityBuildFailureReason(err); got != "build_setup_provider" {
		t.Fatalf("failure reason=%q", got)
	}
}

func TestCOMP2CProductionReadUsesConversationBundleSlot(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "product_daemon.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	foundRead := false
	foundComposition := false
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "newProductDaemonRunnerWithPreparedDecisions" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				name := ""
				switch target := call.Fun.(type) {
				case *ast.Ident:
					name = target.Name
				case *ast.SelectorExpr:
					name = target.Sel.Name
				}
				switch name {
				case "newProductSharedLocalModel":
					t.Error("production constructs shared local model outside loom-conversation Bundle")
				case "NewCodexConversationClient",
					"NewSystemDeepSeekConversationClient",
					"NewSystemKimiConversationClient",
					"NewSystemMiniMaxConversationClient",
					"NewSystemAnthropicConversationClient",
					"newProductConversationProfileRouterWithPolicy",
					"NewEncryptedPersistentLocalProductChatAPI",
					"NewPersistentLocalProductChatAPI":
					t.Errorf("production constructs %s outside loom-conversation Bundle", name)
				case "newProductReadRouteFactoryFromCore":
					if len(call.Args) == 5 && identifierNamed(call.Args[3], "conversationRouteSlot") {
						foundRead = true
					}
				}
			}
			field, ok := node.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			key, ok := field.Key.(*ast.Ident)
			if !ok {
				return true
			}
			switch key.Name {
			case "conversationSlot":
				if identifierNamed(field.Value, "conversationRouteSlot") {
					foundComposition = true
				}
			}
			return true
		})
	}
	if !foundRead || !foundComposition {
		t.Fatalf("readSlot=%t compositionSlot=%t", foundRead, foundComposition)
	}
}

func TestCOMP2CConversationConstructionPreservesFailureStage(t *testing.T) {
	for _, test := range []struct {
		err  error
		want string
	}{
		{errors.Join(errProductConversationNativeAuthConstruction, errors.New("private")), "build_setup_native_auth"},
		{errors.Join(errProductConversationProviderConstruction, errors.New("private")), "build_setup_provider"},
		{errors.Join(errProductConversationRouteUnavailable, errors.New("private")), "build_setup_provider"},
	} {
		if got := productCompatibilityBuildFailureReason(test.err); got != test.want {
			t.Fatalf("failure reason=%q want=%q", got, test.want)
		}
	}
}

func TestCOMP2CProductionConversationCompositionKeepsPiAndOpenCodeRespondersDistinct(t *testing.T) {
	root := t.TempDir()
	executablePath := filepath.Join(root, "opencode")
	if err := os.WriteFile(executablePath, []byte(`#!/bin/sh
printf '%s\n' '{"type":"tool_use","part":{"type":"tool","tool":"StructuredOutput","state":{"status":"completed","input":{"response":"OPENCODE-COMPOSITION-OK","tool_name":"none","tool_arguments":{}}}}}' '{"type":"step_finish","part":{"type":"step-finish"}}'
`), 0o700); err != nil {
		t.Fatal(err)
	}
	diagnostics := &productConversationCompositionDiagnosticFixture{
		now: time.Date(2026, 8, 24, 9, 0, 0, 0, time.UTC),
	}
	factory := newProductConversationConstructionFactory(
		filepath.Join(root, "loom.db"),
		productSetupRuntimeConfig{
			OpenCodeExecutable:    executablePath,
			CredentialLeases:      &profileConversationLeaseRecorder{},
			ConversationResponder: &productPiConversationResponder{},
		},
		projection.New(nil), nil, diagnostics,
	)
	routes, err := factory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := routes.close(); closeErr != nil {
			t.Errorf("close conversation routes: %v", closeErr)
		}
	})
	thread, err := routes.route.SendMessage(
		context.Background(),
		api.LocalProductChatMessageRequest{
			ThreadID: "thread-production-pi-opencode", Content: "hello",
			ProfileID: provider.OpenCodeConversationProfileID,
			ModelID:   provider.OpenCodeConversationDefaultModel,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	last := thread.Messages[len(thread.Messages)-1]
	if last.Role != "loom" || last.Content != "OPENCODE-COMPOSITION-OK" || !last.Tentative {
		t.Fatalf("last message = %#v attempts=%#v diagnostics=%#v", last, thread.Attempts, diagnostics.records)
	}
}

func TestPhase7ProductionConversationBindsMetadataGatewayExactlyOnce(t *testing.T) {
	root := t.TempDir()
	diagnostics := &productConversationCompositionDiagnosticFixture{
		now: time.Date(2026, 8, 28, 14, 0, 0, 0, time.UTC),
	}
	sources := productConversationControlMetadataSources{
		Read: productConversationControlReadFixture{},
	}
	routes, err := newProductConversationConstructionFactory(
		filepath.Join(root, "loom.db"),
		productSetupRuntimeConfig{
			CredentialLeases:      &profileConversationLeaseRecorder{},
			ConversationResponder: &productPiConversationResponder{},
		},
		projection.New(nil), nil, diagnostics, sources,
	)(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := routes.close(); closeErr != nil {
			t.Errorf("close conversation routes: %v", closeErr)
		}
	})
	chat, ok := routes.route.(*api.LocalProductChatAPI)
	if !ok {
		t.Fatalf("production Conversation route type = %T", routes.route)
	}
	second, err := newProductConversationControlMetadataGateway(sources)
	if err != nil {
		t.Fatal(err)
	}
	if err := chat.SetControlMetadataGateway(second); !errors.Is(err, api.ErrInvalidLocalProductChatRequest) {
		t.Fatalf("second metadata Gateway bind err=%v", err)
	}
}

func TestCOMP2CProductionConversationPublishesClaudeCodeThroughHarnessGateway(
	t *testing.T,
) {
	root := t.TempDir()
	executablePath := filepath.Join(root, "claude")
	if err := os.WriteFile(executablePath, []byte(`#!/bin/sh
set -eu
[ "${ANTHROPIC_API_KEY-}" = "" ]
[ "${ANTHROPIC_BASE_URL-}" = "" ]
session_flag=
session_id=
while [ "$#" -gt 0 ]; do
  case "$1" in
    --session-id|--resume)
      session_flag=$1
      session_id=$2
      shift 2
      ;;
    *) shift ;;
  esac
done
[ -n "$session_flag" ]
[ -n "$session_id" ]
printf '%s %s\n' "$session_flag" "$session_id" >> "$0.session-flags"
cat >/dev/null
printf '{"type":"result","subtype":"success","is_error":false,"session_id":"%s","result":"CLAUDE-COMPOSITION-OK","total_cost_usd":"0","usage":{"input_tokens":1,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}\n' "$session_id"
`), 0o700); err != nil {
		t.Fatal(err)
	}
	diagnostics := &productConversationCompositionDiagnosticFixture{
		now: time.Date(2026, 8, 24, 10, 0, 0, 0, time.UTC),
	}
	factory := newProductConversationConstructionFactory(
		filepath.Join(root, "loom.db"),
		productSetupRuntimeConfig{
			ClaudeExecutable: executablePath,
			CredentialLeases: &profileConversationLeaseRecorder{},
		},
		projection.New(nil), nil, diagnostics,
	)
	routes, err := factory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := routes.close(); closeErr != nil {
			t.Errorf("close conversation routes: %v", closeErr)
		}
	})
	thread, err := routes.route.SendMessage(
		context.Background(), api.LocalProductChatMessageRequest{
			ThreadID: "thread-production-claude", Content: "hello",
			ProfileID: provider.ClaudeCodeConversationProfileID,
			ModelID:   provider.AnthropicConversationModelID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	thread, err = routes.route.SendMessage(
		context.Background(), api.LocalProductChatMessageRequest{
			ThreadID: "thread-production-claude", Content: "hello again",
			ProfileID: provider.ClaudeCodeConversationProfileID,
			ModelID:   provider.AnthropicConversationModelID,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	last := thread.Messages[len(thread.Messages)-1]
	if last.Role != "loom" || last.Content != "CLAUDE-COMPOSITION-OK" ||
		!last.Tentative || len(thread.Segments) != 1 || len(thread.Attempts) != 2 ||
		thread.Segments[0].ExecutionBinding == nil ||
		thread.Segments[0].ExecutionBinding.HarnessAdapter != "claude-code" {
		t.Fatalf(
			"last=%#v segments=%#v diagnostics=%#v",
			last, thread.Segments, diagnostics.records,
		)
	}
	var opening *productOperationalDiagnosticRecord
	for index := range diagnostics.records {
		record := &diagnostics.records[index]
		if record.GatewayEventType == string(harnessgateway.EventSessionOpening) {
			opening = record
			break
		}
	}
	if opening == nil || opening.ThreadID != thread.ThreadID ||
		opening.HarnessID != string(harnessgateway.HarnessClaudeCode) ||
		opening.BackendID != string(productClaudeCodeSegmentBackendID) ||
		opening.GatewayEventSchemaVersion != harnessgateway.EventSchemaVersion ||
		opening.GatewayConfiguredHarnessVersion != productHarnessGatewayConfiguredVersion ||
		opening.GatewayBackendVersion != productClaudeCodeSegmentBackendVersion ||
		opening.GatewayEventSequence == 0 || opening.SessionID == "" ||
		opening.SegmentID != thread.Segments[0].SegmentID || opening.ResponseID != "" ||
		!validProductOperationalDiagnosticRecord(*opening) {
		t.Fatalf("Claude Code Gateway opening = %#v diagnostics=%#v", opening, diagnostics.records)
	}
	flags, err := os.ReadFile(executablePath + ".session-flags")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Fields(string(flags))
	if len(lines) != 4 || lines[0] != "--session-id" || lines[2] != "--resume" ||
		lines[1] == "" || lines[1] != lines[3] {
		t.Fatalf(
			"Claude Code native Session flags = %q thread=%#v diagnostics=%#v",
			flags, thread, diagnostics.records,
		)
	}
}

func TestCOMP2CProductionConversationPublishesGovernedHarnessesThroughGateway(
	t *testing.T,
) {
	t.Run("Codex", func(t *testing.T) {
		root := t.TempDir()
		executablePath := filepath.Join(root, "codex")
		if err := os.WriteFile(executablePath, []byte(`#!/bin/sh
set -eu
turn=0
while IFS= read -r line; do
  case "$line" in
    *'"method":"initialize"'*)
      printf '%s\n' '{"id":"loom-initialize-v1","result":{"userAgent":"codex-cli/0.144.1","codexHome":"/private/codex","platformFamily":"unix","platformOs":"darwin"}}'
      ;;
    *'"method":"thread/start"'*)
      printf '%s\n' '{"id":"loom-thread-start-v1","result":{"thread":{"id":"thread-production-codex"},"model":"gpt-5.6-sol"}}'
      ;;
    *'"method":"turn/start"'*)
      turn=$((turn + 1))
      printf '{"id":"loom-turn-start-%s","result":{"turn":{"id":"turn-%s","status":"inProgress"}}}\n' "$turn" "$turn"
      printf '{"method":"thread/tokenUsage/updated","params":{"threadId":"thread-production-codex","turnId":"turn-%s","tokenUsage":{"last":{"inputTokens":1,"cachedInputTokens":0,"outputTokens":1,"reasoningOutputTokens":0,"totalTokens":2}}}}\n' "$turn"
      printf '{"method":"item/completed","params":{"threadId":"thread-production-codex","turnId":"turn-%s","item":{"type":"agentMessage","text":"{\\\"response\\\":\\\"CODEX-GATEWAY-OK\\\",\\\"tool_name\\\":\\\"none\\\",\\\"tool_arguments_json\\\":\\\"{}\\\"}"}}}\n' "$turn"
      printf '{"method":"turn/completed","params":{"threadId":"thread-production-codex","turn":{"id":"turn-%s","status":"completed"}}}\n' "$turn"
      ;;
  esac
done
`), 0o700); err != nil {
			t.Fatal(err)
		}
		assertCOMP2CProductionGovernedGatewayRoute(
			t,
			filepath.Join(root, "loom.db"),
			productSetupRuntimeConfig{
				CodexExecutable:  executablePath,
				CredentialLeases: &profileConversationLeaseRecorder{},
			},
			projection.New(nil),
			api.LocalProductChatMessageRequest{
				ThreadID: "thread-production-codex-gateway", Content: "codex governed input",
				ProfileID: provider.CodexConversationProfileID, ModelID: "codex-default",
			},
			harnessgateway.HarnessCodex,
			"CODEX-GATEWAY-OK",
		)
	})

	t.Run("OpenCode", func(t *testing.T) {
		root := t.TempDir()
		executablePath := filepath.Join(root, "opencode")
		if err := os.WriteFile(executablePath, []byte(`#!/bin/sh
printf '%s\n' '{"type":"tool_use","part":{"type":"tool","tool":"StructuredOutput","state":{"status":"completed","input":{"response":"OPENCODE-GATEWAY-OK","tool_name":"none","tool_arguments":{}}}}}' '{"type":"step_finish","part":{"type":"step-finish"}}'
`), 0o700); err != nil {
			t.Fatal(err)
		}
		assertCOMP2CProductionGovernedGatewayRoute(
			t,
			filepath.Join(root, "loom.db"),
			productSetupRuntimeConfig{
				OpenCodeExecutable: executablePath,
				CredentialLeases:   &profileConversationLeaseRecorder{},
			},
			projection.New(nil),
			api.LocalProductChatMessageRequest{
				ThreadID: "thread-production-opencode-gateway", Content: "opencode input secret",
				ProfileID: provider.OpenCodeConversationProfileID,
				ModelID:   provider.OpenCodeConversationDefaultModel,
			},
			harnessgateway.HarnessOpenCode,
			"OPENCODE-GATEWAY-OK",
		)
	})

	t.Run("Pi", func(t *testing.T) {
		root := t.TempDir()
		content := "review the configured runtime"
		piPath := filepath.Join(root, "pi")
		replyPrompt := productPiConversationFixturePrompt(
			t, "thread-production-pi-gateway", "segment-1", content,
		)
		if err := os.WriteFile(
			piPath, []byte(productPiConversationFixtureScript(
				productPiConversationFixtureEnvelope("", content), replyPrompt,
			)), 0o700,
		); err != nil {
			t.Fatal(err)
		}
		modelConfig := piadapter.PiLocalModelCatalogConfig{
			PrivateRoot:    filepath.Join(root, "local-model"),
			ExecutablePath: filepath.Join(root, "llama-server"),
			ModelPath:      filepath.Join(root, "model.gguf"),
		}
		assertCOMP2CProductionGovernedGatewayRoute(
			t,
			filepath.Join(root, "loom.db"),
			productSetupRuntimeConfig{
				CredentialLeases:            &profileConversationLeaseRecorder{},
				ConversationContextCapsules: &productGatewaySegmentAcceptanceCapsuleStore{},
				Execution: &productMissionExecutionRuntimeConfig{
					RuntimeSearchPaths: []string{root}, RuntimeInstanceID: "runtime-pi-production",
					LocalModelCatalog: &modelConfig,
					LocalModelRuntime: &productSharedLocalModelFixture{},
					Now: func() time.Time {
						return time.Date(2026, 8, 24, 11, 0, 0, 0, time.UTC)
					},
					Random: bytes.NewReader(bytes.Repeat([]byte{0x45}, 2_048)),
				},
			},
			projection.New(nil),
			api.LocalProductChatMessageRequest{
				ThreadID: "thread-production-pi-gateway", Content: content,
				ProfileID: provider.PiConversationProfileID,
			},
			harnessgateway.HarnessPi,
			"Hello world",
		)
	})

	t.Run("LoomNative", func(t *testing.T) {
		_, statePath := productDaemonFailureState(t)
		readModel, database, revision := productCOMP2CVerifiedDeepSeekProjection(
			t, statePath,
		)
		t.Cleanup(func() {
			if err := database.Close(); err != nil {
				t.Errorf("close projection database: %v", err)
			}
		})
		var requestCount atomic.Int32
		server := httptest.NewTLSServer(http.HandlerFunc(func(
			writer http.ResponseWriter,
			request *http.Request,
		) {
			if request.Method != http.MethodPost || request.URL.Path != "/chat/completions" ||
				request.Header.Get("Authorization") != "Bearer account-private-key" {
				http.Error(writer, "invalid bounded request", http.StatusBadRequest)
				return
			}
			body, err := io.ReadAll(request.Body)
			if err != nil {
				http.Error(writer, "invalid bounded request", http.StatusBadRequest)
				return
			}
			call := requestCount.Add(1)
			writer.Header().Set("Content-Type", "application/json")
			switch call {
			case 1:
				payload := string(body)
				if !strings.Contains(payload, `"tool_choice":"required"`) ||
					!strings.Contains(payload, `"name":"loom_conversation_reply"`) {
					http.Error(writer, "typed selection missing", http.StatusBadRequest)
					return
				}
				_, _ = writer.Write([]byte(
					`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call-direct-1","type":"function","function":{"name":"loom_conversation_reply","arguments":"{\"decision\":\"no_product_tool_matches\"}"}}]},"finish_reason":"tool_calls"}]}`,
				))
			case 2:
				payload := string(body)
				if !strings.Contains(payload, `"tools"`) ||
					!strings.Contains(payload, `"tool_choice":"required"`) ||
					!strings.Contains(payload, `"name":"loom_conversation_reply"`) ||
					strings.Contains(payload, `call-direct-1`) ||
					!strings.Contains(payload, `independent typed recheck`) {
					http.Error(writer, "direct reply typed recheck invalid", http.StatusBadRequest)
					return
				}
				_, _ = writer.Write([]byte(
					`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":null,"tool_calls":[{"id":"call-direct-2","type":"function","function":{"name":"loom_conversation_reply","arguments":"{\"decision\":\"no_product_tool_matches\"}"}}]},"finish_reason":"tool_calls"}]}`,
				))
			case 3:
				payload := string(body)
				if strings.Contains(payload, `"tools"`) ||
					strings.Contains(payload, `"tool_choice"`) ||
					strings.Contains(payload, `call-direct-1`) ||
					strings.Contains(payload, `call-direct-2`) ||
					!strings.Contains(payload, `answer_request_directly`) {
					http.Error(writer, "direct answer retained tool authority", http.StatusBadRequest)
					return
				}
				_, _ = writer.Write([]byte(
					`{"model":"deepseek-chat","choices":[{"message":{"role":"assistant","content":"LOOM-NATIVE-GATEWAY-OK"},"finish_reason":"stop"}]}`,
				))
			default:
				http.Error(writer, "unexpected request", http.StatusBadRequest)
			}
		}))
		defer server.Close()

		transport := server.Client().Transport.(*http.Transport).Clone()
		transport.Proxy = nil
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
		transport.TLSClientConfig.ServerName = server.Certificate().DNSNames[0]
		transport.DialContext = func(
			ctx context.Context, network string, address string,
		) (net.Conn, error) {
			if address != "api.deepseek.com:443" {
				return nil, errors.New("bounded fixture rejected external network")
			}
			return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
		}
		originalTransport := http.DefaultTransport
		http.DefaultTransport = transport
		defer func() { http.DefaultTransport = originalTransport }()

		assertCOMP2CProductionGovernedGatewayRoute(
			t,
			statePath,
			productSetupRuntimeConfig{
				CredentialLeases: &profileConversationLeaseRecorder{},
			},
			readModel,
			api.LocalProductChatMessageRequest{
				ThreadID:  "thread-production-loom-native-gateway",
				Content:   "loom native input secret",
				ProfileID: provider.DeepSeekConversationProfileID(revision),
				ModelID:   provider.DeepSeekConversationModelID,
			},
			harnessgateway.HarnessLoomNative,
			"LOOM-NATIVE-GATEWAY-OK",
		)
		if got := requestCount.Load(); got != 3 {
			t.Fatalf("bounded Loom Native Provider request count = %d, want 3", got)
		}
	})
}

func assertCOMP2CProductionGovernedGatewayRoute(
	t *testing.T,
	statePath string,
	setup productSetupRuntimeConfig,
	readModel *projection.Projection,
	request api.LocalProductChatMessageRequest,
	wantHarness harnessgateway.HarnessID,
	wantContent string,
) {
	t.Helper()
	diagnostics := &productConversationCompositionDiagnosticFixture{
		now: time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC),
	}
	routes, err := newProductConversationConstructionFactory(
		statePath, setup, readModel, nil, diagnostics,
	)(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := routes.close(); closeErr != nil {
			t.Errorf("close conversation routes: %v", closeErr)
		}
	})
	thread, err := routes.route.SendMessage(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if len(thread.Messages) != 2 || len(thread.Segments) != 1 || len(thread.Attempts) != 1 {
		t.Fatalf("production thread = %#v", thread)
	}
	last := thread.Messages[len(thread.Messages)-1]
	if last.Role != "loom" || last.Content != wantContent || !last.Tentative ||
		thread.Segments[0].ExecutionBinding == nil ||
		thread.Segments[0].ExecutionBinding.HarnessAdapter != string(wantHarness) {
		t.Fatalf(
			"production response = %#v segment=%#v diagnostics=%#v",
			last, thread.Segments[0], diagnostics.records,
		)
	}

	wantTypes := []harnessgateway.EventType{
		harnessgateway.EventSessionOpening,
		harnessgateway.EventSessionReady,
		harnessgateway.EventResponseStarted,
		harnessgateway.EventResponseCompleted,
	}
	gatewayRecords := make([]productOperationalDiagnosticRecord, 0, len(wantTypes))
	for _, record := range diagnostics.records {
		if record.GatewayEventType != "" {
			gatewayRecords = append(gatewayRecords, record)
		}
	}
	if len(gatewayRecords) != len(wantTypes) {
		t.Fatalf(
			"production responder bypassed registered Gateway: diagnostics=%#v",
			diagnostics.records,
		)
	}
	wantBackend := "backend.segment." + string(wantHarness)
	wantBackendVersion := productHarnessGatewayBackendVersion
	switch wantHarness {
	case harnessgateway.HarnessCodex:
		wantBackend = string(productCodexSegmentBackendID)
	case harnessgateway.HarnessClaudeCode:
		wantBackend = string(productClaudeCodeSegmentBackendID)
	case harnessgateway.HarnessOpenCode:
		wantBackend = string(productOpenCodeSegmentBackendID)
	case harnessgateway.HarnessPi:
		wantBackend = string(productPiSegmentBackendID)
	case harnessgateway.HarnessLoomNative:
		wantBackend = string(productLoomNativeSegmentBackendID)
	}
	for index, record := range gatewayRecords {
		if record.GatewayEventType != string(wantTypes[index]) ||
			record.GatewayEventSchemaVersion != harnessgateway.EventSchemaVersion ||
			record.GatewayConfiguredHarnessVersion != productHarnessGatewayConfiguredVersion ||
			record.GatewayBackendVersion != wantBackendVersion ||
			record.HarnessID != string(wantHarness) || record.BackendID != wantBackend ||
			record.ThreadID != thread.ThreadID || record.SegmentID != thread.Segments[0].SegmentID ||
			record.SessionID == "" || record.WorkspaceID == "" || record.WorkspaceDigest == "" ||
			record.GatewayEventSequence != uint64(index+1) ||
			!validProductOperationalDiagnosticRecord(record) {
			t.Fatalf("Gateway diagnostic %d = %#v", index, record)
		}
	}
	encoded, err := json.Marshal(gatewayRecords)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{request.Content, wantContent, "account-private-key"} {
		if strings.Contains(string(encoded), forbidden) {
			t.Fatalf("Gateway diagnostics leaked content %q: %s", forbidden, encoded)
		}
	}
}

func productCOMP2CVerifiedDeepSeekProjection(
	t *testing.T,
	statePath string,
) (*projection.Projection, *sql.DB, int64) {
	t.Helper()
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	writer, err := state.NewLocalProductSetupWriter(journal.NewStore(database))
	if err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 24, 11, 30, 0, 0, time.UTC)
	configured, err := writer.CommitCredentialMetadata(
		context.Background(), credentials.MetadataCommand{
			CommandID: "configure-deepseek-composition-gateway", ProviderID: "deepseek",
			ProviderAccountID:   "deepseek.primary",
			CredentialReference: "credential-ref-deepseek-composition-gateway",
			ExpectedRevision:    0, OccurredAt: now,
			Status: credentials.CredentialConfigured,
		},
	)
	if err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	verified, err := writer.CommitCredentialMetadata(
		context.Background(), credentials.MetadataCommand{
			CommandID: "verify-deepseek-composition-gateway", ProviderID: "deepseek",
			ProviderAccountID:   "deepseek.primary",
			CredentialReference: configured.CredentialReference,
			ExpectedRevision:    configured.Revision, OccurredAt: now.Add(time.Second),
			Status: credentials.CredentialVerified,
		},
	)
	if err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	readModel := projection.New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	return readModel, database, verified.Revision
}

func TestCOMP2CConversationOwnsSharedLocalModelForAgentRuntime(t *testing.T) {
	shared := &productSharedLocalModelFixture{}
	slot := &productConversationRouteSlot{}
	if err := slot.Bind(productConversationRoutes{
		route: productConversationRouteFixture{}, localModel: shared,
		close: shared.Close,
	}); err != nil {
		t.Fatal(err)
	}
	localModel, err := slot.LocalModelRuntime()
	if err != nil || localModel != shared {
		t.Fatalf("local model = %T err=%v", localModel, err)
	}
	if err := slot.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	if shared.closed != 1 {
		t.Fatalf("shared model closed %d times", shared.closed)
	}
}

type productSharedLocalModelFixture struct{ closed int }

type productConversationCompositionDiagnosticFixture struct {
	now     time.Time
	records []productOperationalDiagnosticRecord
}

func (fixture *productConversationCompositionDiagnosticFixture) operationalNow() time.Time {
	return fixture.now
}

func (*productConversationCompositionDiagnosticFixture) credentialRuntimeValue() string {
	return productCredentialRuntimeExplicitLegacy
}

func (fixture *productConversationCompositionDiagnosticFixture) append(
	record productOperationalDiagnosticRecord,
) error {
	fixture.records = append(fixture.records, record)
	return nil
}

func (*productSharedLocalModelFixture) BaseURL(context.Context) (string, error) {
	return "http://127.0.0.1:18427/v1", nil
}

func (fixture *productSharedLocalModelFixture) Close() error {
	fixture.closed++
	return nil
}

type productConversationRouteFixture struct{}

func (productConversationRouteFixture) ChatThread(
	_ context.Context,
	threadID string,
) (api.LocalProductChatThread, error) {
	return api.LocalProductChatThread{ThreadID: threadID}, nil
}

func (productConversationRouteFixture) SendMessage(
	_ context.Context,
	request api.LocalProductChatMessageRequest,
) (api.LocalProductChatThread, error) {
	return api.LocalProductChatThread{
		ThreadID: request.ThreadID,
		Messages: []api.LocalProductChatMessage{{
			Role: "user", Content: request.Content,
		}},
	}, nil
}

func (productConversationRouteFixture) DeleteThread(
	_ context.Context,
	threadID string,
) error {
	return nil
}
