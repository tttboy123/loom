package main

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/projection"
)

func TestCOMP2CSetupUnavailableClearsCredentialSecret(t *testing.T) {
	secret := []byte("setup-secret")
	if _, err := (&productSetupRouteSlot{}).ConfigureCredential(
		context.Background(), app.CredentialSetupCommand{Secret: secret},
	); err == nil {
		t.Fatal("unbound Setup slot accepted a credential")
	}
	assertProductBytesCleared(t, "Setup credential", secret)
}

func TestCOMP2CSetupConstructionPreservesCredentialFailureStage(t *testing.T) {
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
	slot := &productSetupRouteSlot{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"cccccccc-cccc-4ccc-8ccc-cccccccccccc",
		nil,
		productCompatibilityConstruction{
			setupSlot: slot,
			setupFactory: newProductSetupRouteFactory(
				database, store, readModel, productSetupRuntimeConfig{},
			),
		},
	)
	if facade != nil || err == nil || slot.Ready() {
		t.Fatalf("facade=%#v err=%v ready=%t", facade, err, slot.Ready())
	}
	if got := productCompatibilityBuildFailureReason(err); got != "build_setup_credential" {
		t.Fatalf("failure reason=%q err=%v", got, err)
	}
	var buildFailure *daemonBuildFailure
	if !errors.As(err, &buildFailure) || buildFailure.reason != "build_setup_credential" {
		t.Fatalf("build failure=%#v err=%v", buildFailure, err)
	}
}

func TestCOMP2CSetupConstructsInLocalIPCBundleAndRevokesRoute(t *testing.T) {
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
	slot := &productSetupRouteSlot{}
	constructed := 0
	factory := newProductSetupRouteFactory(
		database, store, readModel,
		productSetupRuntimeConfig{CredentialMutator: &productCredentialTestMutator{}},
	)
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb",
		nil,
		productCompatibilityConstruction{
			setupSlot: slot,
			setupFactory: func(ctx context.Context) (productSetupRoutes, error) {
				constructed++
				return factory(ctx)
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if constructed != 1 || !slot.Ready() {
		t.Fatalf("constructed=%d ready=%t", constructed, slot.Ready())
	}
	if _, err := slot.SetupSnapshot(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	if slot.Ready() {
		t.Fatal("Setup slot remained ready after Composition close")
	}
	if _, err := slot.SetupSnapshot(context.Background()); err == nil {
		t.Fatal("closed Setup slot served a snapshot")
	}
}

func TestCOMP2CProductionDoesNotConstructSetupOutsideBundle(t *testing.T) {
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
				if target, named := call.Fun.(*ast.Ident); named &&
					target.Name == "buildProductSetupService" {
					t.Error("production constructs Setup outside loom-local-ipc Bundle")
				}
			}
			field, ok := node.(*ast.KeyValueExpr)
			if !ok {
				return true
			}
			key, ok := field.Key.(*ast.Ident)
			if ok && key.Name == "setupSlot" && identifierNamed(field.Value, "setupRouteSlot") {
				foundSlot = true
			}
			return true
		})
	}
	if !foundSlot {
		t.Fatal("production Composition does not bind the Setup route slot")
	}
}
