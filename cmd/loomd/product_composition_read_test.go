package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/projection"
)

func TestCOMP2CReadConstructsAfterGovernanceAndRevokesRoute(t *testing.T) {
	_, statePath := productDaemonFailureState(t)
	database, err := openProductReadDatabase(statePath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	store := journal.NewStore(database)
	readModel := projection.New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	governanceSlot := &productGovernanceRouteSlot{}
	readSlot := &productReadRouteSlot{}
	chatSlot := &productConversationRouteSlot{}
	diagnosticSlot := &productObservabilityRouteSlot{}
	chatSlot.routes = productConversationRoutes{
		route: productConversationRouteFixture{}, close: func() error { return nil },
	}
	chatSlot.bound = true
	diagnosticSlot.routes = productObservabilityRoutes{
		contextRetrieval: &productOperationalDiagnosticStore{},
		agentAttempts:    &productOperationalDiagnosticStore{},
		attemptSource:    &productOperationalDiagnosticStore{},
		toolAttempts:     &productOperationalDiagnosticStore{},
		ipc:              &productOperationalDiagnosticStore{},
	}
	diagnosticSlot.bound = true
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"dddddddd-dddd-4ddd-8ddd-dddddddddddd",
		nil,
		productCompatibilityConstruction{
			governanceSlot: governanceSlot,
			governanceFactory: newProductGovernanceRouteFactory(
				store, readModel, nil, app.PreparedMissionDecisions{},
			),
			readSlot: readSlot,
			readFactory: newProductReadRouteFactory(
				store, readModel, governanceSlot, &productRuntimeObservationHealth{},
				chatSlot, diagnosticSlot,
			),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !governanceSlot.Ready() || !readSlot.Ready() {
		t.Fatalf("governance=%t read=%t", governanceSlot.Ready(), readSlot.Ready())
	}
	if _, err := readSlot.ReadLocalProductSnapshot(
		context.Background(), api.LocalProductSnapshotRequest{Limit: 1},
	); err != nil {
		t.Fatal(err)
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	if readSlot.Ready() {
		t.Fatal("Read slot remained ready after Composition close")
	}
}

func TestCOMP2CProductionDoesNotConstructReadOutsideBundle(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "product_daemon.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	foundSlot := false
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "newProductDaemonRunnerWithPreparedDecisions" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if ok {
				selector, selected := call.Fun.(*ast.SelectorExpr)
				if selected && selector.Sel.Name == "NewLocalProductReadService" {
					t.Error("production constructs Read outside Governance lifecycle")
				}
			}
			field, ok := node.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			key, ok := field.Key.(*ast.Ident)
			if ok && key.Name == "readSlot" && identifierNamed(field.Value, "readRouteSlot") {
				foundSlot = true
			}
			return true
		})
	}
	if !foundSlot {
		t.Fatal("production Composition does not bind the Read route slot")
	}
}
