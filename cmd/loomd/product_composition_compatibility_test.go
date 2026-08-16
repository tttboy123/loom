package main

import (
	"bytes"
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/localipc"
)

func TestCOMP2DProductionConstructsLocalIPCHandlerInsideBundle(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "product_daemon.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	foundSlot := false
	foundFactory := false
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "newProductDaemonRunnerWithPreparedDecisions" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				if target, named := call.Fun.(*ast.Ident); named &&
					target.Name == "newProductRouteHandler" {
					t.Error("production constructs the local IPC handler before Bundle activation")
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
			case "localIPCSlot":
				foundSlot = true
			case "localIPCFactory":
				foundFactory = true
			}
			return true
		})
	}
	if !foundSlot || !foundFactory {
		t.Fatalf("production local IPC Bundle construction slot=%t factory=%t", foundSlot, foundFactory)
	}
}

func TestCOMP2ACompatibilityProfilesCompileExactFacade(t *testing.T) {
	wantBundles := []string{
		"loom-core", "loom-assets", "loom-observability", "loom-vault",
		"loom-conversation", "loom-governance", "loom-work", "loom-agent-runtime",
		"loom-local-ipc",
	}
	handler := localipc.HandlerFunc(func(_ context.Context, request localipc.Request) localipc.Response {
		return localipc.Response{
			Version: request.Version, RequestID: request.RequestID, OK: true,
			Result: []byte(`{"facade":"legacy"}`),
		}
	})
	for _, profileID := range []composition.ProfileID{
		composition.ProfileDesktop, composition.ProfileHeadless, composition.ProfileTest,
	} {
		t.Run(string(profileID), func(t *testing.T) {
			recorder := &compositionTestRecorder{}
			facade, err := activateProductCompatibilityComposition(
				context.Background(), profileID, handler,
				"incident-comp2a-"+string(profileID), recorder,
			)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = facade.Close() }()
			snapshot := facade.Snapshot()
			bundleIDs := make([]string, len(snapshot.Bundles))
			for index, descriptor := range snapshot.Bundles {
				bundleIDs[index] = descriptor.ID
			}
			if !reflect.DeepEqual(bundleIDs, wantBundles) {
				t.Fatalf("bundles=%v want=%v", bundleIDs, wantBundles)
			}
			if snapshot.Profile.ID != profileID ||
				!reflect.DeepEqual(snapshot.Routes, productRouteManifest()) ||
				snapshot.Digest == "" {
				t.Fatalf("snapshot=%#v", snapshot)
			}
			if bytes.Contains(snapshot.Canonical, []byte(`"facade":"legacy"`)) ||
				bytes.Contains(snapshot.Canonical, []byte(`"status":"same"`)) {
				t.Fatalf("snapshot captured handler content: %s", snapshot.Canonical)
			}
			if !recorder.has(composition.StageCompositionCompile) ||
				!recorder.has(composition.StageBundleReady) ||
				!recorder.has(composition.StageScopeOpen) {
				t.Fatalf("diagnostics=%#v", recorder.snapshot())
			}
		})
	}
}

func TestCOMP2ACompatibilityFacadeDelegatesCanonicalResponse(t *testing.T) {
	want := localipc.Response{
		Version: 1, RequestID: "request-comp2a", JourneyID: "journey-comp2a",
		OK: true, Result: []byte(`{"status":"same"}`),
	}
	legacy := localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
		return want
	})
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest, legacy,
		"incident-comp2a-parity", nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	request := localipc.Request{
		Version: 1, RequestID: "request-comp2a", JourneyID: "journey-comp2a",
		Method: "setup_snapshot", Params: []byte(`{}`),
	}
	admitted := facade.Handler()
	got := admitted.Handle(context.Background(), request)
	if !reflect.DeepEqual(got, want) || !bytes.Equal(got.Result, want.Result) {
		t.Fatalf("got=%#v want=%#v", got, want)
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	if facade.Handler() != nil {
		t.Fatal("closed facade retained request admission")
	}
	if response := admitted.Handle(context.Background(), request); response.OK ||
		response.Error == nil || response.Error.Code != "state_unavailable" {
		t.Fatalf("previous handler reference bypassed closed composition: %#v", response)
	}
}

func TestCOMP2ACompatibilityFacadeRejectsInvalidAdmission(t *testing.T) {
	if _, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileID("unknown"),
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{}
		}), "incident-comp2a-invalid", nil,
	); err == nil {
		t.Fatal("unknown profile was admitted")
	}
	if _, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest, nil,
		"incident-comp2a-invalid", nil,
	); err == nil {
		t.Fatal("nil legacy handler was admitted")
	}
}

func TestCOMP2ACloseQuiescesAdmittedHandlerBeforeRevokingOldReference(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	legacy := localipc.HandlerFunc(func(_ context.Context, request localipc.Request) localipc.Response {
		close(entered)
		<-release
		return localipc.Response{Version: request.Version, RequestID: request.RequestID, OK: true}
	})
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest, legacy,
		"incident-comp2a-quiesce", nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	admitted := facade.Handler()
	request := localipc.Request{Version: 1, RequestID: "request-quiesce"}
	responseDone := make(chan localipc.Response, 1)
	go func() { responseDone <- admitted.Handle(context.Background(), request) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("handler did not enter")
	}
	closeDone := make(chan error, 1)
	go func() { closeDone <- facade.Close() }()
	select {
	case err := <-closeDone:
		t.Fatalf("Close detached in-flight handler: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if response := <-responseDone; !response.OK {
		t.Fatalf("admitted request changed result: %#v", response)
	}
	if err := <-closeDone; err != nil {
		t.Fatal(err)
	}
	if response := admitted.Handle(context.Background(), request); response.OK ||
		response.Error == nil || response.Error.Code != "state_unavailable" {
		t.Fatalf("closed handler admitted request: %#v", response)
	}
}

func TestCOMP2ACompositionDiagnosticsUseOperationalStore(t *testing.T) {
	root := t.TempDir()
	stateDirectory := filepath.Join(root, "state")
	if err := os.Mkdir(stateDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	store, err := newProductOperationalDiagnosticStore(
		filepath.Join(stateDirectory, "loom.db"), 128<<10,
		func() time.Time { return time.Date(2026, 8, 14, 5, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	recorder, err := newProductCompositionDiagnosticRecorder(store)
	if err != nil {
		t.Fatal(err)
	}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}), "incident-comp2a-operational", recorder,
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	records, err := readProductOperationalDiagnosticFile(store.path, store.maximum)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) == 0 {
		t.Fatal("composition diagnostics were not persisted")
	}
	stages := make(map[string]bool)
	for _, record := range records {
		stages[record.Stage] = true
		if record.Operation != "composition" || record.IncidentID != "incident-comp2a-operational" ||
			record.ProfileID != string(composition.ProfileTest) || record.CompositionSnapshotDigest == "" {
			t.Fatalf("record=%#v", record)
		}
	}
	for _, stage := range []composition.DiagnosticStage{
		composition.StageCompositionCompile, composition.StageCompositionValidate,
		composition.StageRouteCompile, composition.StageBundleRegister,
		composition.StageBundleStart, composition.StageBundleReady,
		composition.StageBundleStop, composition.StageBundleDispose,
		composition.StageScopeOpen, composition.StageScopeClose,
	} {
		if !stages[string(stage)] {
			t.Fatalf("missing stage %s in %#v", stage, records)
		}
	}
	invalid := records[0]
	invalid.ProviderID = "deepseek"
	if validProductOperationalDiagnosticRecord(invalid) {
		t.Fatal("composition diagnostic admitted Provider identity")
	}
}

func TestCOMP2ADiagnosticPersistenceFailurePreventsAdmission(t *testing.T) {
	root := t.TempDir()
	store, err := newProductOperationalDiagnosticStore(
		filepath.Join(root, "loom.db"), 256,
		func() time.Time { return time.Date(2026, 8, 14, 5, 0, 0, 0, time.UTC) },
	)
	if err != nil {
		t.Fatal(err)
	}
	recorder, err := newProductCompositionDiagnosticRecorder(store)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			called = true
			return localipc.Response{OK: true}
		}), "incident-comp2a-diagnostic-failure", recorder,
	)
	if err == nil || facade != nil || called {
		t.Fatalf("facade=%#v called=%t err=%v", facade, called, err)
	}
}

func TestCOMP2AProductionBuilderActivatesDesktopFacade(t *testing.T) {
	root, statePath := productDaemonFailureState(t)
	isolationRoot := filepath.Join(root, "isolation")
	runtimeRoot := filepath.Join(root, "runtime")
	for _, directory := range []string{isolationRoot, runtimeRoot} {
		if err := os.Mkdir(directory, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	runner, err := productionDaemonBuilder(daemonBuildConfig{
		Observer: app.LocalRuntimeObservationDaemonConfig{
			StatePath: statePath, IsolationRoot: isolationRoot,
			RuntimeSearchPaths: []string{runtimeRoot}, ProbeID: "comp2a-probe",
			RuntimeInstanceID: "comp2a-runtime", DeviceID: "comp2a-device",
			DisplayName: "COMP2A", ObservationInterval: time.Hour,
			ProcessTimeout: time.Second, MaxCycles: 1,
		},
		SocketPath: filepath.Join(root, "loomd.sock"),
	})
	if err != nil {
		t.Fatal(err)
	}
	product, ok := runner.(*productDaemonRunner)
	if !ok {
		_ = runner.Close()
		t.Fatalf("runner=%T", runner)
	}
	defer product.Close()
	if product.composition == nil || product.composition.Handler() == nil ||
		product.composition.Snapshot().Profile.ID != composition.ProfileDesktop ||
		product.composition.productScope == nil ||
		product.composition.productScope.Kind() != composition.ScopeProduct {
		t.Fatalf("composition=%#v", product.composition)
	}
	records, err := readProductOperationalDiagnosticFile(
		filepath.Join(root, "diagnostics", "operational.jsonl"),
		productOperationalDiagnosticsMaximum,
	)
	if err != nil {
		t.Fatal(err)
	}
	foundReady := false
	foundProductScope := false
	for _, record := range records {
		if record.Operation == "composition" && record.Stage == string(composition.StageBundleReady) {
			foundReady = true
		}
		if record.Operation == "composition" && record.Stage == string(composition.StageScopeOpen) &&
			record.ScopeKind == string(composition.ScopeProduct) && record.ScopeID == "loom-product" {
			foundProductScope = true
		}
	}
	if !foundReady || !foundProductScope {
		t.Fatalf("composition diagnostics missing before Run: %#v", records)
	}
}

func TestCOMP2DCompatibilityFacadeOwnsProductScope(t *testing.T) {
	recorder := &compositionTestRecorder{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}), "incident-comp2d-product-scope", recorder,
	)
	if err != nil {
		t.Fatal(err)
	}
	if facade.productScope == nil || facade.productScope.Kind() != composition.ScopeProduct ||
		facade.productScope.ID() != "loom-product" ||
		facade.productScope.CompositionSnapshotDigest() != facade.Snapshot().Digest ||
		facade.productScope.ExecutionBindingDigest() != "" {
		t.Fatalf("product scope=%#v snapshot=%#v", facade.productScope, facade.Snapshot())
	}
	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	records := recorder.snapshot()
	var productOpen, productClose bool
	for _, record := range records {
		if record.ScopeKind != composition.ScopeProduct || record.ScopeID != "loom-product" {
			continue
		}
		switch record.Stage {
		case composition.StageScopeOpen:
			productOpen = record.Result == "succeeded"
		case composition.StageScopeClose:
			productClose = record.Result == "succeeded"
		}
	}
	if !productOpen || !productClose {
		t.Fatalf("product scope diagnostics=%#v", records)
	}
}

func TestCOMP2DProductScopeCloseIsConcurrentAndIdempotent(t *testing.T) {
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileTest,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}), "incident-comp2d-concurrent-close", nil,
	)
	if err != nil {
		t.Fatal(err)
	}
	errorsFound := make(chan error, 16)
	var group sync.WaitGroup
	for index := 0; index < cap(errorsFound); index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			errorsFound <- facade.Close()
		}()
	}
	group.Wait()
	close(errorsFound)
	for closeErr := range errorsFound {
		if closeErr != nil {
			t.Fatal(closeErr)
		}
	}
}

type compositionTestRecorder struct {
	mu      sync.Mutex
	records []composition.DiagnosticRecord
}

func (recorder *compositionTestRecorder) RecordCompositionDiagnostic(
	_ context.Context,
	record composition.DiagnosticRecord,
) error {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.records = append(recorder.records, record)
	return nil
}

func (recorder *compositionTestRecorder) snapshot() []composition.DiagnosticRecord {
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return append([]composition.DiagnosticRecord(nil), recorder.records...)
}

func (recorder *compositionTestRecorder) has(stage composition.DiagnosticStage) bool {
	for _, record := range recorder.snapshot() {
		if record.Stage == stage {
			return true
		}
	}
	return false
}
