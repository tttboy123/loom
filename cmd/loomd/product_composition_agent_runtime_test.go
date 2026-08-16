package main

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"testing"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/roundtable"
)

func TestCOMP2CAgentRuntimeConstructsAfterAssetsAndRevokesRoutes(t *testing.T) {
	events := make([]string, 0, 4)
	assetsSlot := &productAssetRouteSlot{}
	workSlot := &productWorkRouteSlot{}
	runtimeSlot := &productAgentRuntimeRouteSlot{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}), "incident-comp2c-agent-runtime", nil,
		productCompatibilityConstruction{
			assetSlot: assetsSlot,
			assetFactory: func(context.Context) (productAssetBundle, error) {
				events = append(events, "assets")
				return productAssetBundleFixture(nil), nil
			},
			workSlot: workSlot,
			workFactory: func(context.Context) (productWorkRoutes, error) {
				events = append(events, "work")
				routes := productWorkRoutesFixture()
				routes.close = func() error {
					events = append(events, "work-close")
					return nil
				}
				return routes, nil
			},
			agentRuntimeSlot: runtimeSlot,
			agentRuntimeFactory: func(context.Context) (productAgentRuntimeRoutes, error) {
				events = append(events, "agent-runtime")
				return productAgentRuntimeRoutes{
					mission:      productMissionExecutionRouteFixture{},
					handoff:      productHandoffRouteFixture{},
					roundtable:   productRoundtableRouteFixture{},
					materializer: productSavedTeamMaterializerFixture{},
					close: func() error {
						events = append(events, "agent-runtime-close")
						return nil
					},
				}, nil
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(events, []string{"assets", "work", "agent-runtime"}) ||
		!workSlot.Ready() || !runtimeSlot.Ready() {
		t.Fatalf(
			"events=%v workReady=%t runtimeReady=%t",
			events, workSlot.Ready(), runtimeSlot.Ready(),
		)
	}
	if _, err := runtimeSlot.ExecuteMission(
		context.Background(), app.MissionExecutionCommand{},
	); err != nil {
		t.Fatal(err)
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(events, []string{
		"assets", "work", "agent-runtime", "agent-runtime-close", "work-close",
	}) || workSlot.Ready() || runtimeSlot.Ready() {
		t.Fatalf(
			"events=%v workReady=%t runtimeReady=%t",
			events, workSlot.Ready(), runtimeSlot.Ready(),
		)
	}
}

func TestCOMP2CAgentRuntimeFailureRollsBackAssetsBeforeProductScope(t *testing.T) {
	assetsSlot := &productAssetRouteSlot{}
	recorder := &compositionTestRecorder{}
	partialClosed := 0
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}), "incident-comp2c-agent-runtime-failure", recorder,
		productCompatibilityConstruction{
			assetSlot: assetsSlot,
			assetFactory: func(context.Context) (productAssetBundle, error) {
				return productAssetBundleFixture(nil), nil
			},
			workSlot: &productWorkRouteSlot{},
			workFactory: func(context.Context) (productWorkRoutes, error) {
				return productWorkRoutesFixture(), nil
			},
			agentRuntimeSlot: &productAgentRuntimeRouteSlot{},
			agentRuntimeFactory: func(context.Context) (productAgentRuntimeRoutes, error) {
				return productAgentRuntimeRoutes{
					close: func() error { partialClosed++; return nil },
				}, errors.New("runtime construction failed")
			},
		},
	)
	if err == nil || facade != nil || assetsSlot.Ready() || partialClosed != 1 {
		t.Fatalf(
			"facade=%#v err=%v assetsReady=%t partialClosed=%d",
			facade, err, assetsSlot.Ready(), partialClosed,
		)
	}
	for _, record := range recorder.snapshot() {
		if record.ScopeKind == composition.ScopeProduct && record.Stage == composition.StageScopeOpen {
			t.Fatalf("Product scope opened after Runtime failure: %#v", recorder.snapshot())
		}
	}
}

func TestCOMP2CProductionBuilderDoesNotConstructAgentRuntimeOutsideBundle(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "product_daemon.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "newProductDaemonRunnerWithPreparedDecisions" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			identifier, ok := call.Fun.(*ast.Ident)
			if ok && identifier.Name == "buildProductMissionExecutionAPI" {
				t.Error("production constructs Agent Runtime outside loom-agent-runtime Bundle")
			}
			return true
		})
	}
}

type productMissionExecutionRouteFixture struct{}

func (productMissionExecutionRouteFixture) ExecuteMission(
	context.Context,
	app.MissionExecutionCommand,
) (api.MissionExecutionEnvelope, error) {
	return api.MissionExecutionEnvelope{}, nil
}

type productHandoffRouteFixture struct{}

func (productHandoffRouteFixture) ProposeSideTask(
	context.Context, app.SideTaskProposalRequest,
) (app.SideTaskProposalResult, error) {
	return app.SideTaskProposalResult{}, nil
}
func (productHandoffRouteFixture) CreateSideTask(
	context.Context, app.SideTaskCreateRequest,
) (app.SideTaskCreateResult, error) {
	return app.SideTaskCreateResult{}, nil
}
func (productHandoffRouteFixture) ReadSideTask(
	context.Context, app.SideTaskReadRequest,
) (app.SideTaskReadResult, error) {
	return app.SideTaskReadResult{}, nil
}
func (productHandoffRouteFixture) DecideSideTask(
	context.Context, app.SideTaskDecisionRequest,
) (app.SideTaskDecisionResult, error) {
	return app.SideTaskDecisionResult{}, nil
}

type productSavedTeamMaterializerFixture struct{}

func (productSavedTeamMaterializerFixture) MaterializeConfirmedTeam(
	_ context.Context,
	confirmation app.BuilderConfirmation,
) (app.BuilderConfirmation, error) {
	return confirmation, nil
}

type productRoundtableRouteFixture struct{}

func (productRoundtableRouteFixture) CreateSession(
	context.Context, roundtable.CreateSessionCommand,
) (roundtable.View, error) {
	return roundtable.View{}, nil
}
func (productRoundtableRouteFixture) AddSeat(
	context.Context, roundtable.AddSeatCommand,
) (roundtable.View, error) {
	return roundtable.View{}, nil
}
func (productRoundtableRouteFixture) RetireSeat(
	context.Context, roundtable.RetireSeatCommand,
) (roundtable.View, error) {
	return roundtable.View{}, nil
}
func (productRoundtableRouteFixture) OpenRound(
	context.Context, roundtable.OpenRoundCommand,
) (roundtable.View, error) {
	return roundtable.View{}, nil
}
func (productRoundtableRouteFixture) ProposeMessage(
	context.Context, roundtable.ProposeMessageCommand,
) (roundtable.View, error) {
	return roundtable.View{}, nil
}
func (productRoundtableRouteFixture) RelayMessage(
	context.Context, roundtable.RelayMessageCommand,
) (roundtable.View, error) {
	return roundtable.View{}, nil
}
func (productRoundtableRouteFixture) AcknowledgeMessage(
	context.Context, roundtable.AcknowledgeMessageCommand,
) (roundtable.View, error) {
	return roundtable.View{}, nil
}
func (productRoundtableRouteFixture) InsertMessage(
	context.Context, roundtable.InsertMessageCommand,
) (roundtable.View, error) {
	return roundtable.View{}, nil
}
func (productRoundtableRouteFixture) DropMessage(
	context.Context, roundtable.DropMessageCommand,
) (roundtable.View, error) {
	return roundtable.View{}, nil
}
func (productRoundtableRouteFixture) ConcludeSession(
	context.Context, roundtable.ConcludeSessionCommand,
) (roundtable.View, error) {
	return roundtable.View{}, nil
}
func (productRoundtableRouteFixture) ReadView(
	context.Context, string,
) (roundtable.View, error) {
	return roundtable.View{}, nil
}

var _ productRoundtableRoute = productRoundtableRouteFixture{}
