package main

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/localipc"
)

func TestCOMP2CAssetsConstructInsideBundleStart(t *testing.T) {
	slot := &productAssetRouteSlot{}
	constructed := 0
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}), "incident-comp2c-assets", nil,
		productCompatibilityConstruction{
			assetSlot: slot,
			assetFactory: func(context.Context) (productAssetBundle, error) {
				constructed++
				return productAssetBundleFixture(nil), nil
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if constructed != 1 || !slot.Ready() {
		t.Fatalf("constructed=%d ready=%t", constructed, slot.Ready())
	}
	result, err := slot.EvolutionAssetSnapshot(
		context.Background(), api.EvolutionAssetSnapshotRequest{},
	)
	if err != nil || result.ViewVersion != "fixture-assets" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	if slot.Ready() {
		t.Fatal("closed composition retained Assets route")
	}
	if _, err := slot.EvolutionAssetSnapshot(
		context.Background(), api.EvolutionAssetSnapshotRequest{},
	); !errors.Is(err, api.ErrInvalidLocalProductAssetAPI) {
		t.Fatalf("closed route err=%v", err)
	}
}

func TestCOMP2CAssetsConstructionFailurePreventsProductScope(t *testing.T) {
	recorder := &compositionTestRecorder{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}), "incident-comp2c-assets-failure", recorder,
		productCompatibilityConstruction{
			assetSlot: &productAssetRouteSlot{},
			assetFactory: func(context.Context) (productAssetBundle, error) {
				return productAssetBundle{}, errors.New("asset construction failed")
			},
		},
	)
	if err == nil || facade != nil {
		t.Fatalf("facade=%#v err=%v", facade, err)
	}
	var failedStart, productOpen bool
	for _, record := range recorder.snapshot() {
		if record.BundleID == "loom-assets" && record.Stage == composition.StageBundleStart &&
			record.Result == "failed" {
			failedStart = true
		}
		if record.ScopeKind == composition.ScopeProduct && record.Stage == composition.StageScopeOpen {
			productOpen = true
		}
	}
	if !failedStart || productOpen {
		t.Fatalf("failedStart=%t productOpen=%t records=%#v", failedStart, productOpen, recorder.snapshot())
	}
}

func TestCOMP2CProductionBuilderDoesNotConstructAssetRouteOutsideBundle(t *testing.T) {
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
			if identifier, ok := node.(*ast.Ident); ok &&
				(identifier.Name == "assetAuthority" || identifier.Name == "assetEvidenceStore") {
				t.Errorf("production retains Assets owner %s outside loom-assets Bundle", identifier.Name)
			}
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if ok && (selector.Sel.Name == "NewLocalProductAssetService" ||
				selector.Sel.Name == "NewLocalProductAssetAPI") {
				t.Errorf("production constructs %s outside loom-assets Bundle", selector.Sel.Name)
			}
			return true
		})
	}
}

func TestCOMP2CAssetsDisposeWaitsForInFlightRoute(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	slot := &productAssetRouteSlot{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}), "incident-comp2c-assets-quiescence", nil,
		productCompatibilityConstruction{
			assetSlot: slot,
			assetFactory: func(context.Context) (productAssetBundle, error) {
				return productAssetBundleFixture(
					&blockingProductAssetRoute{entered: entered, release: release},
				), nil
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	callDone := make(chan error, 1)
	go func() {
		_, callErr := slot.EvolutionAssetSnapshot(
			context.Background(), api.EvolutionAssetSnapshotRequest{},
		)
		callDone <- callErr
	}()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("Assets route did not enter")
	}
	closeDone := make(chan error, 1)
	go func() { closeDone <- facade.Close() }()
	select {
	case closeErr := <-closeDone:
		t.Fatalf("Close bypassed in-flight Assets route: %v", closeErr)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if callErr := <-callDone; callErr != nil {
		t.Fatal(callErr)
	}
	if closeErr := <-closeDone; closeErr != nil {
		t.Fatal(closeErr)
	}
}

func TestCOMP2CAssetsPublishesBoundedMaterializerAndOwnsItsClose(t *testing.T) {
	closed := 0
	slot := &productAssetRouteSlot{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}), "incident-comp2c-assets-port", nil,
		productCompatibilityConstruction{
			assetSlot: slot,
			assetFactory: func(context.Context) (productAssetBundle, error) {
				bundle := productAssetBundleFixture(nil)
				bundle.close = func() error { closed++; return nil }
				return bundle, nil
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	materializer, err := slot.TeamAssetMaterializer()
	if err != nil || materializer == nil {
		t.Fatalf("materializer=%#v err=%v", materializer, err)
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	if closed != 1 {
		t.Fatalf("asset close count=%d", closed)
	}
	if _, err := slot.TeamAssetMaterializer(); !errors.Is(err, api.ErrInvalidLocalProductAssetAPI) {
		t.Fatalf("closed materializer port err=%v", err)
	}
}

func productAssetBundleFixture(route productAssetRoute) productAssetBundle {
	if route == nil {
		route = productAssetRouteFixture{}
	}
	return productAssetBundle{
		route:        route,
		materializer: productTeamAssetMaterializerFixture{},
		close:        func() error { return nil },
	}
}

type productAssetRouteFixture struct{}

func (productAssetRouteFixture) EvolutionAssetSnapshot(
	context.Context,
	api.EvolutionAssetSnapshotRequest,
) (api.EvolutionAssetSnapshot, error) {
	return api.EvolutionAssetSnapshot{ViewVersion: "fixture-assets"}, nil
}

func (productAssetRouteFixture) EvolutionAssetDiff(
	context.Context,
	api.EvolutionAssetDiffRequest,
) (api.EvolutionAssetDiff, error) {
	return api.EvolutionAssetDiff{}, nil
}

func (productAssetRouteFixture) EvolutionAssetCommand(
	context.Context,
	api.EvolutionAssetCommandRequest,
) (api.EvolutionAssetCommandResult, error) {
	return api.EvolutionAssetCommandResult{}, nil
}

var _ productAssetRoute = productAssetRouteFixture{}

type productTeamAssetMaterializerFixture struct{}

func (productTeamAssetMaterializerFixture) PrepareTeamAttemptMaterialization(
	context.Context,
	app.TeamAssetMaterializationRequest,
) (app.TeamAssetMaterialization, error) {
	return app.TeamAssetMaterialization{}, nil
}

func (productTeamAssetMaterializerFixture) CleanupTeamAttemptMaterialization(
	context.Context,
	app.TeamAssetMaterialization,
) error {
	return nil
}

type blockingProductAssetRoute struct {
	once    sync.Once
	entered chan struct{}
	release chan struct{}
}

func (route *blockingProductAssetRoute) EvolutionAssetSnapshot(
	context.Context,
	api.EvolutionAssetSnapshotRequest,
) (api.EvolutionAssetSnapshot, error) {
	route.once.Do(func() { close(route.entered) })
	<-route.release
	return api.EvolutionAssetSnapshot{}, nil
}

func (*blockingProductAssetRoute) EvolutionAssetDiff(
	context.Context,
	api.EvolutionAssetDiffRequest,
) (api.EvolutionAssetDiff, error) {
	return api.EvolutionAssetDiff{}, nil
}

func (*blockingProductAssetRoute) EvolutionAssetCommand(
	context.Context,
	api.EvolutionAssetCommandRequest,
) (api.EvolutionAssetCommandResult, error) {
	return api.EvolutionAssetCommandResult{}, nil
}
