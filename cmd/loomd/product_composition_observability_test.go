package main

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"
	"time"

	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/localipc"
)

func TestCOMP2CObservabilityPersistentStoreFailureOccursInsideBundleStart(
	t *testing.T,
) {
	root := t.TempDir()
	stateDirectory := filepath.Join(root, "state")
	if err := os.Mkdir(stateDirectory, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "diagnostics"), []byte("blocked"), 0o600); err != nil {
		t.Fatal(err)
	}
	bootstrap, err := newProductOperationalDiagnosticBootstrap(
		filepath.Join(stateDirectory, "loom.db"),
		productOperationalDiagnosticsMaximum,
		func() time.Time { return time.Unix(1_723_600_000, 0).UTC() },
	)
	if err != nil {
		t.Fatal(err)
	}
	recorder, err := newProductCompositionDiagnosticRecorder(bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	slot := &productObservabilityRouteSlot{}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileDesktop,
		localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa",
		recorder,
		productCompatibilityConstruction{
			observabilitySlot:    slot,
			observabilityFactory: newProductObservabilityConstructionFactory(bootstrap),
		},
	)
	if facade != nil || err == nil || slot.Ready() {
		t.Fatalf("facade=%#v err=%v ready=%t", facade, err, slot.Ready())
	}
	if got := productCompatibilityBuildFailureReason(err); got != "build_diagnostics" {
		t.Fatalf("failure reason=%q", got)
	}
}

func TestCOMP2CObservabilityRejectsTypedNilDiagnosticSink(t *testing.T) {
	var store *productOperationalDiagnosticStore
	if recorder, err := newProductCompositionDiagnosticRecorder(store); err == nil || recorder != nil {
		t.Fatalf("recorder=%#v err=%v", recorder, err)
	}
}

func TestCOMP2CObservabilityBootstrapHandoffBindsAndRevokesOperationalPorts(
	t *testing.T,
) {
	_, statePath := productDaemonFailureState(t)
	bootstrap, err := newProductOperationalDiagnosticBootstrap(
		statePath, productOperationalDiagnosticsMaximum,
		func() time.Time { return time.Unix(1_723_600_000, 0).UTC() },
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := bootstrap.setCredentialRuntime(productCredentialRuntimeVault); err != nil {
		t.Fatal(err)
	}
	slot := &productObservabilityRouteSlot{}
	constructed := 0
	factory := newProductObservabilityConstructionFactory(bootstrap)
	recorder, err := newProductCompositionDiagnosticRecorder(bootstrap)
	if err != nil {
		t.Fatal(err)
	}
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileDesktop,
		localipc.HandlerFunc(func(
			context.Context, localipc.Request,
		) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"11111111-1111-4111-8111-111111111111",
		recorder,
		productCompatibilityConstruction{
			observabilitySlot: slot,
			observabilityFactory: func(
				ctx context.Context,
			) (productObservabilityRoutes, error) {
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
	store, ok := slot.routes.ipc.(*productOperationalDiagnosticStore)
	if !ok || store == nil {
		t.Fatal("Observability Bundle did not bind its persistent store")
	}

	wrapped := slot.wrap(localipc.HandlerFunc(func(
		context.Context, localipc.Request,
	) localipc.Response {
		return localipc.Response{OK: true}
	}))
	request := localipc.Request{
		Version:   1,
		RequestID: "22222222-2222-4222-8222-222222222222",
		Method:    "credential_vault_lock",
		Params:    []byte(`{}`),
	}
	if response := wrapped.Handle(context.Background(), request); !response.OK {
		t.Fatalf("response = %#v", response)
	}
	records, err := readProductOperationalDiagnosticFile(
		store.path, productOperationalDiagnosticsMaximum,
	)
	if err != nil || len(records) < 2 {
		t.Fatalf("records = %#v, %v", records, err)
	}
	foundCompileBeforeStart := false
	foundRequest := false
	requestRecords := 0
	for _, record := range records {
		if record.Operation == "composition" && record.Stage == "composition_compile" {
			foundCompileBeforeStart = true
		}
		if record.IncidentID == request.RequestID && record.Operation == request.Method &&
			record.Stage == "credential_lease_revoke" {
			foundRequest = true
			requestRecords++
		}
	}
	if !foundCompileBeforeStart || !foundRequest {
		t.Fatalf("compile=%t request=%t records=%#v", foundCompileBeforeStart, foundRequest, records)
	}
	if diagnostics, err := slot.AgentAttemptDiagnostics(
		context.Background(), nil,
	); err != nil || len(diagnostics) != 0 {
		t.Fatalf("diagnostics = %#v, %v", diagnostics, err)
	}

	if err := facade.Close(); err != nil {
		t.Fatal(err)
	}
	if slot.Ready() {
		t.Fatal("observability slot remained ready after Composition close")
	}
	if _, err := slot.AgentAttemptDiagnostics(context.Background(), nil); err == nil {
		t.Fatal("closed observability slot accepted a diagnostic query")
	}
	if response := wrapped.Handle(context.Background(), request); !response.OK {
		t.Fatalf("closed wrapped response = %#v", response)
	}
	recordsAfterClose, err := readProductOperationalDiagnosticFile(
		store.path, productOperationalDiagnosticsMaximum,
	)
	requestRecordsAfterClose := 0
	for _, record := range recordsAfterClose {
		if record.IncidentID == request.RequestID && record.Operation == request.Method {
			requestRecordsAfterClose++
		}
	}
	if err != nil || requestRecordsAfterClose != requestRecords {
		t.Fatalf("records after close = %#v, %v", recordsAfterClose, err)
	}
}

func TestCOMP2CObservabilityBootstrapHandoffFailsActivationAtomically(
	t *testing.T,
) {
	slot := &productObservabilityRouteSlot{}
	want := errors.New("observability construction failed")
	facade, err := activateProductCompatibilityComposition(
		context.Background(), composition.ProfileDesktop,
		localipc.HandlerFunc(func(
			context.Context, localipc.Request,
		) localipc.Response {
			return localipc.Response{OK: true}
		}),
		"33333333-3333-4333-8333-333333333333",
		&compositionTestRecorder{},
		productCompatibilityConstruction{
			observabilitySlot: slot,
			observabilityFactory: func(
				context.Context,
			) (productObservabilityRoutes, error) {
				return productObservabilityRoutes{}, want
			},
		},
	)
	if facade != nil || !errors.Is(err, want) || slot.Ready() {
		t.Fatalf("facade=%#v err=%v ready=%t", facade, err, slot.Ready())
	}
	if got := productCompatibilityBuildFailureReason(err); got != "build_diagnostics" {
		t.Fatalf("failure reason=%q", got)
	}
}

func TestCOMP2CProductionUsesObservabilitySlotAfterBootstrap(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "product_daemon.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "newProductDaemonRunnerWithPreparedDecisions" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			switch current := node.(type) {
			case *ast.SelectorExpr:
				if identifierNamed(current.X, "observabilityRouteSlot") &&
					current.Sel.Name == "wrap" {
					found["ipc"] = true
				}
			case *ast.CallExpr:
				switch target := current.Fun.(type) {
				case *ast.Ident:
					if target.Name == "newProductOperationalDiagnosticStore" {
						t.Errorf("production constructs operational store outside loom-observability Bundle")
					}
					if target.Name == "newProductOperationalDiagnosticBootstrap" {
						found["bootstrap"] = true
					}
					if target.Name == "newProductObservabilityConstructionFactory" &&
						len(current.Args) == 1 && identifierNamed(current.Args[0], "operationalDiagnostics") {
						found["factory"] = true
					}
					if target.Name == "newProductReadRouteFactoryFromCore" && len(current.Args) == 5 &&
						identifierNamed(current.Args[4], "observabilityRouteSlot") {
						found["read"] = true
					}
				case *ast.SelectorExpr:
					if identifierNamed(target.X, "operationalDiagnostics") &&
						target.Sel.Name != "setCredentialRuntime" {
						t.Errorf("bootstrap store retains downstream method %s", target.Sel.Name)
					}
				}
			case *ast.AssignStmt:
				for index, right := range current.Rhs {
					if !identifierNamed(right, "observabilityRouteSlot") || index >= len(current.Lhs) {
						continue
					}
					selector, ok := current.Lhs[index].(*ast.SelectorExpr)
					if !ok {
						continue
					}
					switch selector.Sel.Name {
					case "ContextRetrievalAuditor":
						found["context"] = true
					case "Diagnostics":
						found["agent"] = true
					}
				}
			case *ast.KeyValueExpr:
				key, ok := current.Key.(*ast.Ident)
				if ok && key.Name == "observabilitySlot" &&
					identifierNamed(current.Value, "observabilityRouteSlot") {
					found["composition"] = true
				}
			}
			return true
		})
	}
	for _, boundary := range []string{"bootstrap", "factory", "read", "ipc", "context", "agent", "composition"} {
		if !found[boundary] {
			t.Errorf("production observability boundary %q is not routed through Bundle slot", boundary)
		}
	}
}

func identifierNamed(expression ast.Expr, name string) bool {
	identifier, ok := expression.(*ast.Ident)
	return ok && identifier.Name == name
}
