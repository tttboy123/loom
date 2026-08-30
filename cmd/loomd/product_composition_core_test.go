package main

import (
	"context"
	"database/sql"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/localipc"
)

func TestCOMP2CCoreConstructsProtectedStateInsideBundleAndClosesIt(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.db")
	prepareProductCoreTestState(t, statePath)
	slot := &productCoreRouteSlot{}
	factory := newProductCoreRouteFactory(productCoreConstructionConfig{
		StatePath: statePath,
		Prepared:  app.PreparedMissionDecisions{},
	})
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"incident-comp2c-core", nil,
		productCompatibilityConstruction{coreSlot: slot, coreFactory: factory},
	)
	if err != nil {
		t.Fatal(err)
	}
	resources, err := slot.Resources()
	if err != nil || resources.database == nil || resources.store == nil ||
		resources.readModel == nil {
		t.Fatalf("resources=%#v err=%v", resources, err)
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	if slot.Ready() {
		t.Fatal("closed Composition retained protected core resources")
	}
	if _, err := slot.Resources(); err == nil {
		t.Fatal("closed core slot exposed Journal/Projection resources")
	}
	if err := resources.database.PingContext(context.Background()); err == nil {
		t.Fatal("loom-core Effect did not close the database")
	}
}

func TestCOMP2CCoreStartsBeforeDependentBundleFactory(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.db")
	prepareProductCoreTestState(t, statePath)
	coreSlot := &productCoreRouteSlot{}
	assetSlot := &productAssetRouteSlot{}
	assetSawCore := false
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"incident-comp2c-core-order", nil,
		productCompatibilityConstruction{
			coreSlot: coreSlot,
			coreFactory: newProductCoreRouteFactory(productCoreConstructionConfig{
				StatePath: statePath,
				Prepared:  app.PreparedMissionDecisions{},
			}),
			assetSlot: assetSlot,
			assetFactory: func(context.Context) (productAssetBundle, error) {
				_, resourceErr := coreSlot.Resources()
				assetSawCore = resourceErr == nil
				return productAssetBundleFixture(nil), resourceErr
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = facade.Close() }()
	if !assetSawCore || !coreSlot.Ready() || !assetSlot.Ready() {
		t.Fatalf("assetSawCore=%t coreReady=%t assetReady=%t",
			assetSawCore, coreSlot.Ready(), assetSlot.Ready())
	}
}

func TestCOMP2CLaterBundleFailureRollsBackProtectedCore(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.db")
	prepareProductCoreTestState(t, statePath)
	coreSlot := &productCoreRouteSlot{}
	assetSlot := &productAssetRouteSlot{}
	var opened productCoreResources
	want := errors.New("asset construction failed")
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"incident-comp2c-core-rollback", nil,
		productCompatibilityConstruction{
			coreSlot: coreSlot,
			coreFactory: newProductCoreRouteFactory(productCoreConstructionConfig{
				StatePath: statePath,
			}),
			assetSlot: assetSlot,
			assetFactory: func(context.Context) (productAssetBundle, error) {
				var resourceErr error
				opened, resourceErr = coreSlot.Resources()
				return productAssetBundle{}, errors.Join(want, resourceErr)
			},
		},
	)
	if facade != nil || !errors.Is(err, want) || coreSlot.Ready() || assetSlot.Ready() {
		t.Fatalf("facade=%#v err=%v coreReady=%t assetReady=%t",
			facade, err, coreSlot.Ready(), assetSlot.Ready())
	}
	if opened.database == nil {
		t.Fatal("dependent Bundle never observed core resources")
	}
	if err := opened.database.PingContext(context.Background()); err == nil {
		t.Fatal("activation rollback retained the protected core database")
	}
}

func TestCOMP2CCoreConstructionPreservesBuildFailureStage(t *testing.T) {
	slot := &productCoreRouteSlot{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"incident-comp2c-core-stage", nil,
		productCompatibilityConstruction{
			coreSlot: slot,
			coreFactory: newProductCoreRouteFactory(productCoreConstructionConfig{
				StatePath: filepath.Join(t.TempDir(), "missing.db"),
			}),
		},
	)
	if facade != nil || err == nil || slot.Ready() {
		t.Fatalf("facade=%#v err=%v ready=%t", facade, err, slot.Ready())
	}
	if got := productCompatibilityBuildFailureReason(err); got != "build_state" {
		t.Fatalf("failure reason=%q", got)
	}
}

func prepareProductCoreTestState(t *testing.T, statePath string) {
	t.Helper()
	database, err := sql.Open("sqlite", statePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := journal.Migrate(context.Background(), database); err != nil {
		_ = database.Close()
		t.Fatal(err)
	}
	if err := database.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCOMP2CProductionBuilderDoesNotConstructProtectedCoreOutsideBundle(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "product_daemon.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	forbiddenCalls := map[string]bool{
		"openProductReadDatabase":                       true,
		"controlledRuntimeStatusFixtureFromEnvironment": true,
		"controlledMissionFixtureFromEnvironment":       true,
		"ensureProductVerifiedNativeAgentRuntimes":      true,
		"ensureProductVerifiedClaudeCodeAgentRuntime":   true,
		"ensureProductVerifiedCodexAgentRuntime":        true,
		"newProductLegacyCredentialLeaseAccess":         true,
	}
	allowedConstructionCalls := map[string]bool{
		"controlledProductJourneyHarnessFromEnvironment":    true,
		"newProductVaultRouteFactoryFromCore":               true,
		"newProductLegacyCredentialLeaseFactory":            true,
		"newProductCoreRouteFactory":                        true,
		"newProductHarnessRuntimeRefresher":                 true,
		"newProductOperationalDiagnosticBootstrap":          true,
		"newProductObservabilityConstructionFactory":        true,
		"newProductConversationConstructionFactoryFromCore": true,
		"newProductGovernanceRouteFactoryFromCore":          true,
		"newProductReadRouteFactoryFromCore":                true,
		"newProductSetupRouteFactoryFromCore":               true,
		"newProductAssetRouteFactoryFromCore":               true,
		"newProductWorkRouteFactoryFromCore":                true,
		"newProductAgentRuntimeFactoryFromCore":             true,
		"newProductLocalIPCHandlerFactory":                  true,
		"newProductCompositionDiagnosticRecorder":           true,
	}
	foundCoreFactory := false
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
			switch target := call.Fun.(type) {
			case *ast.Ident:
				if forbiddenCalls[target.Name] {
					t.Errorf("production constructs protected core through %s", target.Name)
				}
				constructionLike := strings.HasPrefix(target.Name, "newProduct") ||
					strings.HasPrefix(target.Name, "buildProduct") ||
					strings.HasPrefix(target.Name, "openProduct") ||
					strings.HasPrefix(target.Name, "ensureProduct") ||
					strings.HasPrefix(target.Name, "controlledProduct")
				if constructionLike && !allowedConstructionCalls[target.Name] {
					t.Errorf("production contains unclassified construction call %s", target.Name)
				}
				if target.Name == "newProductCoreRouteFactory" {
					foundCoreFactory = true
				}
			case *ast.SelectorExpr:
				if target.Sel.Name == "NewStore" || target.Sel.Name == "New" &&
					identifierNamed(target.X, "projection") {
					t.Errorf("production constructs protected core through %s", target.Sel.Name)
				}
			}
			return true
		})
	}
	if !foundCoreFactory {
		t.Fatal("production does not delegate protected core construction to loom-core")
	}
}

func TestPhase5CoreStartupDoesNotDiscoverExternalHarnessRuntimes(t *testing.T) {
	parsed, err := parser.ParseFile(
		token.NewFileSet(), "product_composition_core.go", nil, 0,
	)
	if err != nil {
		t.Fatal(err)
	}
	forbidden := map[string]bool{
		"ensureProductVerifiedClaudeCodeAgentRuntime": true,
		"ensureProductVerifiedCodexAgentRuntime":      true,
		"ensureProductVerifiedOpenCodeAgentRuntime":   true,
	}
	foundNative := false
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "newProductCoreRouteFactory" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			identifier, ok := call.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			if forbidden[identifier.Name] {
				t.Errorf("core startup discovers external Harness through %s", identifier.Name)
			}
			if identifier.Name == "ensureProductVerifiedNativeAgentRuntimes" {
				foundNative = true
			}
			return true
		})
	}
	if !foundNative {
		t.Fatal("core startup no longer publishes the immediate Loom Native runtime")
	}
}
