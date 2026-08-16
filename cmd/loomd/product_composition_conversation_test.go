package main

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/localipc"
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
