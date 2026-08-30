package main

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

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

func TestCOMP2CDeferredAgentRuntimeDoesNotBlockProductComposition(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var releaseOnce sync.Once
	runtimeSlot := &productAgentRuntimeRouteSlot{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}), "incident-comp2c-deferred-agent-runtime", nil,
		productCompatibilityConstruction{
			assetSlot: &productAssetRouteSlot{},
			assetFactory: func(context.Context) (productAssetBundle, error) {
				return productAssetBundleFixture(nil), nil
			},
			workSlot: &productWorkRouteSlot{},
			workFactory: func(context.Context) (productWorkRoutes, error) {
				return productWorkRoutesFixture(), nil
			},
			agentRuntimeSlot: runtimeSlot,
			agentRuntimeFactory: func(context.Context) (productAgentRuntimeRoutes, error) {
				close(started)
				<-release
				return productAgentRuntimeRoutes{
					mission:      productMissionExecutionRouteFixture{},
					handoff:      productHandoffRouteFixture{},
					roundtable:   productRoundtableRouteFixture{},
					materializer: productSavedTeamMaterializerFixture{},
					close:        func() error { return nil },
				}, nil
			},
			deferAgentRuntime: true,
		},
	)
	if err != nil || facade == nil {
		t.Fatalf("facade=%#v err=%v", facade, err)
	}
	defer func() {
		releaseOnce.Do(func() { close(release) })
		if closeErr := facade.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("deferred Agent Runtime did not start")
	}
	if runtimeSlot.Ready() || !runtimeSlot.compositionReady() {
		t.Fatalf(
			"runtime ready=%t compositionReady=%t",
			runtimeSlot.Ready(), runtimeSlot.compositionReady(),
		)
	}
	reason := runtimeSlot.UnavailableReason()
	if reason == nil || reason.Error() != "agent runtime unavailable: construction" {
		t.Fatalf("initializing reason = %v", reason)
	}
	releaseOnce.Do(func() { close(release) })
	deadline := time.Now().Add(time.Second)
	for !runtimeSlot.Ready() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !runtimeSlot.Ready() || runtimeSlot.UnavailableReason() != nil {
		t.Fatalf("runtime ready=%t reason=%v", runtimeSlot.Ready(), runtimeSlot.UnavailableReason())
	}
}

func TestCOMP2CDeferredAgentRuntimeCloseCancelsAndJoinsInitialization(t *testing.T) {
	started := make(chan struct{})
	exited := make(chan struct{})
	runtimeSlot := &productAgentRuntimeRouteSlot{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}), "incident-comp2c-deferred-agent-runtime-close", nil,
		productCompatibilityConstruction{
			assetSlot: &productAssetRouteSlot{},
			assetFactory: func(context.Context) (productAssetBundle, error) {
				return productAssetBundleFixture(nil), nil
			},
			workSlot: &productWorkRouteSlot{},
			workFactory: func(context.Context) (productWorkRoutes, error) {
				return productWorkRoutesFixture(), nil
			},
			agentRuntimeSlot: runtimeSlot,
			agentRuntimeFactory: func(ctx context.Context) (productAgentRuntimeRoutes, error) {
				close(started)
				<-ctx.Done()
				close(exited)
				return productDegradedAgentRuntimeRoutes(productUnavailableAgentRuntime{}), ctx.Err()
			},
			deferAgentRuntime: true,
		},
	)
	if err != nil || facade == nil {
		t.Fatalf("facade=%#v err=%v", facade, err)
	}
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("deferred Agent Runtime did not start")
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
	case <-time.After(time.Second):
		t.Fatal("Agent Runtime initialization outlived composition close")
	}
	if runtimeSlot.compositionReady() || runtimeSlot.Ready() {
		t.Fatalf(
			"closed runtime ready=%t compositionReady=%t",
			runtimeSlot.Ready(), runtimeSlot.compositionReady(),
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

func TestCOMP2CDegradedAgentRuntimeDoesNotAdvertiseReady(t *testing.T) {
	diagnostics := &productAgentRuntimeBuildDiagnosticFixture{}
	runtimeSlot := &productAgentRuntimeRouteSlot{}
	construction := productCompatibilityConstruction{
		assetSlot:        &productAssetRouteSlot{},
		assetFactory:     func(context.Context) (productAssetBundle, error) { return productAssetBundle{}, nil },
		workSlot:         &productWorkRouteSlot{},
		workFactory:      func(context.Context) (productWorkRoutes, error) { return productWorkRoutes{}, nil },
		agentRuntimeSlot: runtimeSlot,
		agentRuntimeFactory: func(context.Context) (productAgentRuntimeRoutes, error) {
			return productDegradedAgentRuntimeRoutes(productSavedTeamMaterializerFixture{}),
				newProductAgentRuntimeBuildError(
					"runtime_adapters", errors.New("private provider detail"),
				)
		},
		diagnostics: diagnostics,
		profileID:   composition.ProfileTest,
	}
	effect, err := construction.startAgentRuntime(context.Background(), nil)
	if err != nil || effect == nil {
		t.Fatalf("start effect=%#v err=%v", effect, err)
	}
	if runtimeSlot.Ready() {
		t.Fatal("degraded Agent Runtime advertised ready")
	}
	reason := runtimeSlot.UnavailableReason()
	if reason == nil || reason.Error() != "agent runtime unavailable: runtime_adapters" ||
		strings.Contains(reason.Error(), "private provider detail") ||
		!errors.Is(reason, api.ErrInvalidLocalProductExecutionAPI) {
		t.Fatalf("unavailable reason = %v", reason)
	}
	if readyErr := construction.agentRuntimeReady(context.Background(), nil); readyErr != nil {
		t.Fatalf("degraded Bundle must not block the product composition: %v", readyErr)
	}
	if _, missionErr := runtimeSlot.ExecuteMission(
		context.Background(), app.MissionExecutionCommand{},
	); missionErr != reason {
		t.Fatalf("Mission error = %v, want scoped reason %v", missionErr, reason)
	}
	if len(diagnostics.records) != 1 ||
		diagnostics.records[0].ErrorCode != "agent_runtime_runtime_adapters" {
		t.Fatalf("diagnostics = %#v", diagnostics.records)
	}
	if _, err := runtimeSlot.MaterializeConfirmedTeam(
		context.Background(), app.BuilderConfirmation{},
	); err != nil {
		t.Fatalf("compatibility materializer = %v", err)
	}
	if err := effect.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestCOMP2CDiagnosticAppendFailureDoesNotBlockDegradedAgentRuntime(t *testing.T) {
	diagnostics := &productAgentRuntimeBuildDiagnosticFixture{
		appendErr: errors.New("private diagnostics storage failure"),
	}
	assetsSlot := &productAssetRouteSlot{}
	workSlot := &productWorkRouteSlot{}
	runtimeSlot := &productAgentRuntimeRouteSlot{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}), "incident-comp2c-diagnostic-append-failure", nil,
		productCompatibilityConstruction{
			assetSlot: assetsSlot,
			assetFactory: func(context.Context) (productAssetBundle, error) {
				return productAssetBundleFixture(nil), nil
			},
			workSlot: workSlot,
			workFactory: func(context.Context) (productWorkRoutes, error) {
				return productWorkRoutesFixture(), nil
			},
			agentRuntimeSlot: runtimeSlot,
			agentRuntimeFactory: func(context.Context) (productAgentRuntimeRoutes, error) {
				return productDegradedAgentRuntimeRoutes(productSavedTeamMaterializerFixture{}),
					newProductAgentRuntimeBuildError(
						"runtime_adapters", errors.New("private provider detail"),
					)
			},
			diagnostics: diagnostics,
			profileID:   composition.ProfileTest,
		},
	)
	if err != nil || facade == nil {
		t.Fatalf("facade=%#v err=%v", facade, err)
	}
	if diagnostics.appendCalls != 1 {
		t.Fatalf("diagnostic append calls = %d", diagnostics.appendCalls)
	}
	if !assetsSlot.Ready() || !workSlot.Ready() ||
		!runtimeSlot.compositionReady() || runtimeSlot.Ready() {
		t.Fatalf(
			"assetsReady=%t workReady=%t runtimeBound=%t runtimeReady=%t",
			assetsSlot.Ready(), workSlot.Ready(),
			runtimeSlot.compositionReady(), runtimeSlot.Ready(),
		)
	}
	reason := runtimeSlot.UnavailableReason()
	if reason == nil || reason.Error() != "agent runtime unavailable: runtime_adapters" ||
		strings.Contains(reason.Error(), "private provider detail") ||
		strings.Contains(reason.Error(), diagnostics.appendErr.Error()) ||
		!errors.Is(reason, api.ErrInvalidLocalProductExecutionAPI) {
		t.Fatalf("unavailable reason = %v", reason)
	}
	if _, missionErr := runtimeSlot.ExecuteMission(
		context.Background(), app.MissionExecutionCommand{},
	); missionErr != reason {
		t.Fatalf("Mission error = %v, want scoped reason %v", missionErr, reason)
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCOMP2CDegradedAgentRuntimePreservesTeamMaterialization(t *testing.T) {
	routes := productDegradedAgentRuntimeRoutes(productSavedTeamMaterializerFixture{})
	if !routes.valid() {
		t.Fatalf("degraded routes are not closed: %#v", routes)
	}
	if _, err := routes.mission.ExecuteMission(
		context.Background(), app.MissionExecutionCommand{},
	); !errors.Is(err, api.ErrInvalidLocalProductExecutionAPI) {
		t.Fatalf("degraded Mission error = %v", err)
	}
	confirmation := app.BuilderConfirmation{
		TeamDefinitionID: "team-preserved", TeamDefinitionVersion: 1,
		TeamDefinitionDigest: strings.Repeat("a", 64), Status: "active",
	}
	materialized, err := routes.materializer.MaterializeConfirmedTeam(
		context.Background(), confirmation,
	)
	if err != nil || materialized != confirmation {
		t.Fatalf("materialization = %#v, %v", materialized, err)
	}
}

func TestCOMP2CDegradedAgentRuntimeRecordsSafeBuildStage(t *testing.T) {
	diagnostics := &productAgentRuntimeBuildDiagnosticFixture{}
	if err := recordProductAgentRuntimeBuildFailure(
		diagnostics,
		newProductAgentRuntimeBuildError("runtime_adapters", errors.New("private cause")),
		composition.ProfileTest,
		strings.Repeat("b", 64),
	); err != nil {
		t.Fatal(err)
	}
	if len(diagnostics.records) != 1 {
		t.Fatalf("diagnostics=%#v", diagnostics.records)
	}
	record := diagnostics.records[0]
	if record.Operation != "composition" || record.BundleID != "loom-agent-runtime" ||
		record.Stage != "bundle_start" || record.Result != "failed" ||
		record.ErrorCode != "agent_runtime_runtime_adapters" || !record.Retryable {
		t.Fatalf("diagnostic = %#v", record)
	}
	if !validProductOperationalDiagnosticRecord(record) {
		t.Fatalf("diagnostic is not accepted by the installed store: %#v", record)
	}
}

func TestCOMP2CDeferredAgentRuntimeRecordsSafeCompletionDuration(t *testing.T) {
	diagnostics := &productAgentRuntimeBuildDiagnosticFixture{}
	if err := recordProductAgentRuntimeBuildOutcome(
		diagnostics, nil, composition.ProfileTest,
		strings.Repeat("c", 64), 45*time.Second,
	); err != nil {
		t.Fatal(err)
	}
	if len(diagnostics.records) != 1 {
		t.Fatalf("diagnostics=%#v", diagnostics.records)
	}
	record := diagnostics.records[0]
	if record.Operation != "composition" || record.BundleID != "loom-agent-runtime" ||
		record.Stage != "bundle_start" || record.Result != "succeeded" ||
		record.ErrorCode != "" || record.Retryable || record.ElapsedMS != 45_000 {
		t.Fatalf("diagnostic = %#v", record)
	}
	if !validProductOperationalDiagnosticRecord(record) {
		t.Fatalf("diagnostic is not accepted by the installed store: %#v", record)
	}
}

type productAgentRuntimeBuildDiagnosticFixture struct {
	records     []productOperationalDiagnosticRecord
	appendErr   error
	appendCalls int
}

func (*productAgentRuntimeBuildDiagnosticFixture) operationalNow() time.Time {
	return time.Date(2026, 8, 21, 13, 0, 0, 0, time.UTC)
}

func (*productAgentRuntimeBuildDiagnosticFixture) credentialRuntimeValue() string {
	return productCredentialRuntimeVault
}

func (fixture *productAgentRuntimeBuildDiagnosticFixture) append(
	record productOperationalDiagnosticRecord,
) error {
	fixture.appendCalls++
	if fixture.appendErr != nil {
		return fixture.appendErr
	}
	fixture.records = append(fixture.records, record)
	return nil
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
func (productRoundtableRouteFixture) PauseRound(
	context.Context, roundtable.PauseRoundCommand,
) (roundtable.View, error) {
	return roundtable.View{}, nil
}
func (productRoundtableRouteFixture) SteerSeat(
	context.Context, productRoundtableSteerSeatCommand,
) (roundtable.View, error) {
	return roundtable.View{}, nil
}
func (productRoundtableRouteFixture) RetrySeat(
	context.Context, productRoundtableRetrySeatCommand,
) (roundtable.View, error) {
	return roundtable.View{}, nil
}
func (productRoundtableRouteFixture) SkipSeat(
	context.Context, roundtable.SkipSeatCommand,
) (roundtable.View, error) {
	return roundtable.View{}, nil
}
func (productRoundtableRouteFixture) ReplaceSeat(
	context.Context, productRoundtableReplaceSeatCommand,
) (roundtable.View, error) {
	return roundtable.View{}, nil
}
func (productRoundtableRouteFixture) ExportSession(
	context.Context, productRoundtableExportCommand,
) (roundtable.ExportDocumentResult, error) {
	return roundtable.ExportDocumentResult{}, nil
}
func (productRoundtableRouteFixture) ImportSession(
	context.Context, productRoundtableImportCommand,
) (roundtable.ImportResult, error) {
	return roundtable.ImportResult{}, nil
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
