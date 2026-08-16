package main

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/execution"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/permissions"
)

func TestCOMP2CWorkConstructsQueueInsideBundleStartAndRevokesRoute(t *testing.T) {
	constructed := 0
	closed := 0
	slot := &productWorkRouteSlot{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}), "incident-comp2c-work", nil,
		productCompatibilityConstruction{
			workSlot: slot,
			workFactory: func(context.Context) (productWorkRoutes, error) {
				constructed++
				routes := productWorkRoutesFixture()
				routes.close = func() error { closed++; return nil }
				return routes, nil
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if constructed != 1 || !slot.Ready() {
		t.Fatalf("constructed=%d ready=%t", constructed, slot.Ready())
	}
	result, err := slot.QueueSnapshot(context.Background(), api.QueueSnapshotRequest{})
	if err != nil || result.ViewVersion != "fixture-queue" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	workers, err := slot.WorkersSnapshot(context.Background(), app.WorkersSnapshotRequest{})
	if err != nil || workers.ViewVersion != "fixture-workers" {
		t.Fatalf("workers=%#v err=%v", workers, err)
	}
	integration, err := slot.Snapshot(context.Background())
	if err != nil || integration.ViewVersion != "fixture-integration" {
		t.Fatalf("integration=%#v err=%v", integration, err)
	}
	executionSnapshot, err := slot.ExecutionSnapshot(
		context.Background(), app.ExecutionSnapshotRequest{},
	)
	if err != nil || executionSnapshot.ViewVersion != "fixture-execution" {
		t.Fatalf("execution=%#v err=%v", executionSnapshot, err)
	}
	toolExecution, err := slot.ToolExecution()
	if err != nil || toolExecution == nil {
		t.Fatalf("toolExecution=%#v err=%v", toolExecution, err)
	}
	toolRecovery, err := slot.RecoverToolCall(
		context.Background(),
		productToolRecoveryRequest{
			SchemaVersion: 1, Operation: "preview",
			IncidentID: "incident-comp2c-work-preview",
		},
	)
	if err != nil || toolRecovery.Operation != "preview" {
		t.Fatalf("toolRecovery=%#v err=%v", toolRecovery, err)
	}
	productionSnapshot, err := slot.ProductionSnapshot(
		context.Background(), app.ProductionSnapshotRequest{},
	)
	if err != nil || productionSnapshot.ViewVersion != "fixture-production" {
		t.Fatalf("production=%#v err=%v", productionSnapshot, err)
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	if slot.Ready() {
		t.Fatal("closed composition retained Work route")
	}
	if _, err := slot.QueueSnapshot(
		context.Background(), api.QueueSnapshotRequest{},
	); !errors.Is(err, api.ErrInvalidLocalQueueAPI) {
		t.Fatalf("closed Work route err=%v", err)
	}
	if _, err := slot.ToolExecution(); !errors.Is(err, api.ErrInvalidLocalExecutionAPI) {
		t.Fatalf("closed tool execution port err=%v", err)
	}
	if _, err := slot.RecoverToolCall(
		context.Background(),
		productToolRecoveryRequest{
			SchemaVersion: 1, Operation: "preview",
			IncidentID: "incident-comp2c-work-closed",
		},
	); !errors.Is(err, errProductInvalidToolRecoveryRequest) {
		t.Fatalf("closed tool recovery route err=%v", err)
	}
	if _, err := slot.ProductionSnapshot(
		context.Background(), app.ProductionSnapshotRequest{},
	); !errors.Is(err, api.ErrInvalidLocalProductionAPI) {
		t.Fatalf("closed Production route err=%v", err)
	}
	if !slot.Degraded(context.Background()) {
		t.Fatal("closed Production route did not fail closed as degraded")
	}
	if closed != 1 {
		t.Fatalf("Work owner close count=%d", closed)
	}
	if err := facade.Close(); err != nil || closed != 1 {
		t.Fatalf("second close err=%v count=%d", err, closed)
	}
}

func TestCOMP2CWorkConstructionFailurePreventsProductScope(t *testing.T) {
	recorder := &compositionTestRecorder{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}), "incident-comp2c-work-failure", recorder,
		productCompatibilityConstruction{
			workSlot: &productWorkRouteSlot{},
			workFactory: func(context.Context) (productWorkRoutes, error) {
				return productWorkRoutes{}, errors.New("Work construction failed")
			},
		},
	)
	if err == nil || facade != nil {
		t.Fatalf("facade=%#v err=%v", facade, err)
	}
	var failedStart, productOpen bool
	for _, record := range recorder.snapshot() {
		if record.BundleID == "loom-work" && record.Stage == composition.StageBundleStart &&
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

func TestCOMP2CProductionBuilderDoesNotConstructQueueOutsideWorkBundle(t *testing.T) {
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
				(identifier.Name == "executionEvidenceStore" ||
					identifier.Name == "executionAdapter" ||
					identifier.Name == "boundedExecutionAPI") {
				t.Errorf("production retains Execution owner %s outside loom-work Bundle", identifier.Name)
			}
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if ok && (selector.Sel.Name == "NewLocalQueueService" ||
				selector.Sel.Name == "NewLocalQueueAPI" ||
				selector.Sel.Name == "NewWorkerExecutionService" ||
				selector.Sel.Name == "NewLocalWorkersService" ||
				selector.Sel.Name == "NewLocalWorkersAPI" ||
				selector.Sel.Name == "NewIntegrationService" ||
				selector.Sel.Name == "NewObservabilityService" ||
				selector.Sel.Name == "NewLocalIntegrationService" ||
				selector.Sel.Name == "NewLocalIntegrationAPI" ||
				selector.Sel.Name == "NewAdapter" ||
				selector.Sel.Name == "NewLocalExecutionService" ||
				selector.Sel.Name == "NewLocalExecutionAPI" ||
				selector.Sel.Name == "NewService" ||
				selector.Sel.Name == "NewLocalProductionService" ||
				selector.Sel.Name == "NewLocalProductionAPI") {
				t.Errorf("production constructs %s outside loom-work Bundle", selector.Sel.Name)
			}
			return true
		})
	}
}

func TestCOMP2CWorkConstructionPreservesBuildFailureStage(t *testing.T) {
	for _, test := range []struct {
		name string
		err  error
		want string
	}{
		{name: "queue", err: errors.Join(errProductWorkQueueConstruction, errors.New("private")), want: "build_queue"},
		{name: "workers", err: errors.Join(errProductWorkWorkersConstruction, errors.New("private")), want: "build_workers"},
		{name: "integration", err: errors.Join(errProductWorkIntegrationConstruction, errors.New("private")), want: "build_integration"},
		{name: "execution", err: errors.Join(errProductWorkExecutionConstruction, errors.New("private")), want: "build_execution"},
		{name: "execution recovery", err: errors.Join(errProductWorkExecutionRecovery, errors.New("private")), want: "build_execution_recovery"},
		{name: "production", err: errors.Join(errProductWorkProductionConstruction, errors.New("private")), want: "build_production"},
		{name: "assets", err: api.ErrInvalidLocalProductAssetAPI, want: "build_assets"},
		{name: "runtime", err: api.ErrInvalidLocalProductExecutionAPI, want: "build_execution"},
		{name: "other", err: errors.New("private"), want: "build_ipc"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := productCompatibilityBuildFailureReason(test.err); got != test.want {
				t.Fatalf("failure reason=%q want=%q", got, test.want)
			}
		})
	}
}

func TestCOMP2CWorkProductionPathsPreserveSandboxAndHomeBoundaries(t *testing.T) {
	sandboxRoot := t.TempDir()
	t.Setenv("LOOM_PRODUCTION_SANDBOX_ROOT", sandboxRoot)
	paths, err := productWorkProductionPaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.AppSupport != filepath.Join(sandboxRoot, "Application Support", "Loom") ||
		paths.LaunchAgents != filepath.Join(sandboxRoot, "LaunchAgents") ||
		!filepath.IsAbs(paths.DaemonPath) {
		t.Fatalf("sandbox paths=%#v", paths)
	}

	t.Setenv("LOOM_PRODUCTION_SANDBOX_ROOT", "relative")
	if _, err := productWorkProductionPaths(); err == nil {
		t.Fatal("relative Production sandbox root accepted")
	}

	home := t.TempDir()
	t.Setenv("LOOM_PRODUCTION_SANDBOX_ROOT", "")
	t.Setenv("HOME", home)
	paths, err = productWorkProductionPaths()
	if err != nil {
		t.Fatal(err)
	}
	if paths.AppSupport != filepath.Join(home, "Library", "Application Support", "Loom") ||
		paths.LaunchAgents != filepath.Join(home, "Library", "LaunchAgents") {
		t.Fatalf("home paths=%#v", paths)
	}
}

type productQueueRouteFixture struct{}

func (productQueueRouteFixture) QueueSnapshot(
	context.Context,
	api.QueueSnapshotRequest,
) (api.QueueSnapshot, error) {
	return api.QueueSnapshot{ViewVersion: "fixture-queue"}, nil
}

func (productQueueRouteFixture) QueueCommand(
	context.Context,
	api.QueueCommandRequest,
) (api.QueueCommandResult, error) {
	return api.QueueCommandResult{}, nil
}

type productWorkersRouteFixture struct{}

func (productWorkersRouteFixture) WorkersSnapshot(
	context.Context,
	app.WorkersSnapshotRequest,
) (app.WorkersSnapshot, error) {
	return app.WorkersSnapshot{ViewVersion: "fixture-workers"}, nil
}

func (productWorkersRouteFixture) WorkersCommand(
	context.Context,
	app.WorkersCommandRequest,
) (app.WorkersCommandResult, error) {
	return app.WorkersCommandResult{}, nil
}

type productIntegrationRouteFixture struct{}

func (productIntegrationRouteFixture) Snapshot(
	context.Context,
) (app.IntegrationSnapshot, error) {
	return app.IntegrationSnapshot{ViewVersion: "fixture-integration"}, nil
}

func (productIntegrationRouteFixture) Command(
	context.Context,
	app.IntegrationCommandRequest,
) (app.IntegrationCommandResult, error) {
	return app.IntegrationCommandResult{}, nil
}

type productExecutionRouteFixture struct{}

func (productExecutionRouteFixture) ExecutionSnapshot(
	context.Context,
	app.ExecutionSnapshotRequest,
) (app.ExecutionSnapshot, error) {
	return app.ExecutionSnapshot{ViewVersion: "fixture-execution"}, nil
}

func (productExecutionRouteFixture) ExecutionCommand(
	context.Context,
	app.ExecutionCommandRequest,
) (app.ExecutionCommandResult, error) {
	return app.ExecutionCommandResult{}, nil
}

type productToolExecutionPortFixture struct{}

func (productToolExecutionPortFixture) Execute(
	context.Context,
	execution.Proposal,
) (execution.ExecutionResult, error) {
	return execution.ExecutionResult{}, nil
}

func (productToolExecutionPortFixture) RemoteToolKinds() []permissions.ToolKind {
	return nil
}

type productProductionRouteFixture struct{}

func (productProductionRouteFixture) ProductionSnapshot(
	context.Context,
	app.ProductionSnapshotRequest,
) (api.ProductionSnapshot, error) {
	return api.ProductionSnapshot{ViewVersion: "fixture-production"}, nil
}

func (productProductionRouteFixture) ProductionCommand(
	context.Context,
	app.ProductionCommandRequest,
) (app.ProductionCommandResult, error) {
	return app.ProductionCommandResult{}, nil
}

func (productProductionRouteFixture) Degraded(context.Context) bool { return false }

type productToolRecoveryRouteFixture struct{}

func (productToolRecoveryRouteFixture) RecoverToolCall(
	_ context.Context,
	request productToolRecoveryRequest,
) (productToolRecoveryResponse, error) {
	return productToolRecoveryResponse{
		SchemaVersion: 1,
		IncidentID:    request.IncidentID,
		Operation:     request.Operation,
		Candidates:    []productToolRecoveryCandidate{},
	}, nil
}

func productWorkRoutesFixture() productWorkRoutes {
	return productWorkRoutes{
		queue:         productQueueRouteFixture{},
		workers:       productWorkersRouteFixture{},
		integration:   productIntegrationRouteFixture{},
		execution:     productExecutionRouteFixture{},
		toolExecution: productToolExecutionPortFixture{},
		toolRecovery:  productToolRecoveryRouteFixture{},
		production:    productProductionRouteFixture{},
		close:         func() error { return nil },
	}
}
