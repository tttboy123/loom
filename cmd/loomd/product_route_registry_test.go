package main

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"sort"
	"testing"

	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/localipc"
)

func TestCOMP2BRouteManifestFreezesEveryLegacyMethod(t *testing.T) {
	want := []composition.RouteMethod{
		"agent_attempt_recovery", "agent_input", "builder_answer", "builder_confirm", "builder_edit", "builder_start",
		"builder_validate", "chat_context_disclosure", "chat_message", "chat_response_cancel", "chat_thread", "chat_thread_delete", "codex_connect",
		"credential_configure", "credential_import", "credential_replace", "credential_revoke",
		"credential_vault_export", "credential_vault_lock", "credential_vault_reset",
		"credential_vault_rotate", "credential_vault_unlock", "credential_verify",
		"customer_rule_command", "customer_rule_snapshot", "evolution_asset_command",
		"evolution_asset_diff", "evolution_asset_snapshot", "execution_command",
		"execution_snapshot", "integration_command", "integration_snapshot",
		"mission_decision", "mission_execution", "permissions_attention",
		"permissions_command", "permissions_snapshot", "production_command",
		"production_snapshot", "provider_account_policy_configure", "provider_endpoint_review_approve",
		"provider_failure_lab_run", "provider_model_rate_card_configure", "queue_command", "queue_snapshot",
		"remote_tool_backend_enrollment_configure", "remote_tool_backend_enrollment_revoke",
		"roundtable_ack_message", "roundtable_add_seat", "roundtable_conclude",
		"roundtable_drop_message", "roundtable_insert_message", "roundtable_open_round",
		"roundtable_propose_message", "roundtable_relay_message", "roundtable_retire_seat",
		"roundtable_session_create", "roundtable_snapshot",
		"setup_snapshot", "side_task_handoff", "snapshot", "standing_order_command",
		"standing_order_snapshot", "team_archive", "team_restore", "timeline_page",
		"tool_recovery", "workers_command", "workers_snapshot",
	}
	sort.Slice(want, func(left, right int) bool { return want[left] < want[right] })
	manifest := productRouteManifest()
	got := make([]composition.RouteMethod, len(manifest))
	seen := make(map[composition.RouteMethod]bool, len(manifest))
	for index, route := range manifest {
		got[index] = route.Method
		if seen[route.Method] || route.SchemaVersion != 1 || route.OwnerBundle == "" ||
			route.HandlerCapability.ID == "" || len(route.RequiredCapabilities) == 0 ||
			route.AvailabilityFailure == "" || route.IncidentPolicy == "" ||
			(route.PrivacyClass != composition.PrivacyMetadataOnly &&
				route.PrivacyClass != composition.PrivacyLocalContent) {
			t.Fatalf("invalid route=%#v", route)
		}
		seen[route.Method] = true
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("methods=%v want=%v", got, want)
	}
	governance := composition.CapabilityApprovedToolInvoker.Ref()
	for _, route := range manifest {
		if route.Method == "chat_context_disclosure" &&
			(route.OwnerBundle != "loom-conversation" ||
				route.PrivacyClass != composition.PrivacyMetadataOnly) {
			t.Fatalf("context disclosure route=%#v", route)
		}
		switch route.Method {
		case "provider_endpoint_review_approve",
			"remote_tool_backend_enrollment_configure",
			"remote_tool_backend_enrollment_revoke":
			if route.OwnerBundle != "loom-governance" ||
				route.HandlerCapability != governance ||
				!reflect.DeepEqual(route.RequiredCapabilities, []composition.CapabilityRef{governance}) {
				t.Fatalf("setup governance route=%#v", route)
			}
		}
	}
}

func TestCOMP2EHandlerAndManifestHaveExactMethodParity(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "product_route_handler.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var handlerMethods []composition.RouteMethod
	for _, declaration := range parsed.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Name.Name != "newProductRouteHandler" {
			continue
		}
		ast.Inspect(function.Body, func(node ast.Node) bool {
			switchStatement, ok := node.(*ast.SwitchStmt)
			if !ok || !requestMethodSelector(switchStatement.Tag) {
				return true
			}
			for _, statement := range switchStatement.Body.List {
				clause := statement.(*ast.CaseClause)
				for _, expression := range clause.List {
					literal, ok := expression.(*ast.BasicLit)
					if ok && literal.Kind == token.STRING {
						handlerMethods = append(handlerMethods, composition.RouteMethod(
							literal.Value[1:len(literal.Value)-1],
						))
					}
				}
			}
			return false
		})
	}
	sort.Slice(handlerMethods, func(left, right int) bool { return handlerMethods[left] < handlerMethods[right] })
	manifestMethods := productRouteMethods(productRouteManifest())
	if !reflect.DeepEqual(handlerMethods, manifestMethods) {
		t.Fatalf("handler methods=%v manifest methods=%v", handlerMethods, manifestMethods)
	}
}

func requestMethodSelector(expression ast.Expr) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Method" {
		return false
	}
	identifier, ok := selector.X.(*ast.Ident)
	return ok && identifier.Name == "request"
}

func TestCOMP2BProfilesDigestExactMethodRouteManifest(t *testing.T) {
	handler := localipc.HandlerFunc(func(context.Context, localipc.Request) localipc.Response {
		return localipc.Response{OK: true}
	})
	want := productRouteManifest()
	for _, profileID := range []composition.ProfileID{
		composition.ProfileDesktop, composition.ProfileHeadless, composition.ProfileTest,
	} {
		facade, err := activateProductCompatibilityComposition(
			context.Background(), profileID, handler,
			"incident-comp2b-"+string(profileID), nil,
		)
		if err != nil {
			t.Fatal(err)
		}
		snapshot := facade.Snapshot()
		if err := facade.Close(); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(snapshot.Routes, want) ||
			!reflect.DeepEqual(snapshot.Profile.RequiredRoutes, routeMethods(want)) {
			t.Fatalf("profile=%s routes=%#v want=%#v", profileID, snapshot.Routes, want)
		}
	}
}

func TestCOMP2BTypedRegistryFreezesLegacyAvailability(t *testing.T) {
	registry := newProductRouteRegistry(productRouteServices{})
	want := map[composition.RouteMethod]string{
		"credential_vault_export": "credential_unavailable",
		"credential_vault_lock":   "credential_unavailable",
		"credential_vault_reset":  "credential_unavailable",
		"credential_vault_rotate": "credential_unavailable",
		"credential_vault_unlock": "credential_unavailable",
		"side_task_handoff":       "internal",
	}
	for _, route := range productRouteManifest() {
		if _, found := want[route.Method]; !found {
			want[route.Method] = "state_unavailable"
		}
	}
	if len(registry.routes) != len(want) {
		t.Fatalf("registered=%d want=%d", len(registry.routes), len(want))
	}
	for method, code := range want {
		registration, found := registry.routes[method]
		if !found || registration.descriptor.AvailabilityFailure != code {
			t.Fatalf("method=%s registration=%#v want=%s", method, registration, code)
		}
		response, rejected := registry.admit(context.Background(), localipc.Request{
			Version: 1, RequestID: "request-comp2b", JourneyID: "journey-comp2b",
			Method: string(method),
		})
		if !rejected || response.Error == nil || response.Error.Code != code {
			t.Fatalf("method=%s response=%#v rejected=%v want=%s", method, response, rejected, code)
		}
	}
	response, rejected := registry.admit(context.Background(), localipc.Request{
		Method: "unknown_method",
	})
	if !rejected || response.Error == nil || response.Error.Code != "unknown_method" {
		t.Fatalf("registry did not fail closed for unknown method: response=%#v rejected=%v", response, rejected)
	}
}

func TestCOMP2ETypedHandlerMatchesDeclarativeAvailability(t *testing.T) {
	services := productRouteServices{}
	registry := newProductRouteRegistry(services)
	handler := newProductRouteHandler(services)
	for _, route := range productRouteManifest() {
		request := localipc.Request{
			Version:   1,
			RequestID: "request-comp2e-parity",
			JourneyID: "journey-comp2e-parity",
			Method:    string(route.Method),
			Params:    []byte(`{}`),
		}
		want, rejected := registry.admit(context.Background(), request)
		if !rejected {
			t.Fatalf("method=%s was not rejected by an empty declarative registry", route.Method)
		}
		got := handler(context.Background(), request)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("method=%s handler=%#v registry=%#v", route.Method, got, want)
		}
	}
}

func TestCOMP2ERouteHandlerOwnedOutsideProductDaemon(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "product_daemon.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range parsed.Decls {
		switch typed := declaration.(type) {
		case *ast.FuncDecl:
			switch typed.Name.Name {
			case "newProductRouteHandler":
				t.Fatal("typed route handler remains declared in product_daemon.go")
			case "localProductHandlerWithComposition":
				t.Fatal("legacy positional composition handler remains declared")
			}
		case *ast.GenDecl:
			for _, spec := range typed.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if ok && typeSpec.Name.Name == "productRouteServices" {
					t.Fatal("route services remain declared in product_daemon.go")
				}
			}
		}
	}
}

func TestCOMP2EProductStartupUsesClosedCompositionActivation(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "product_daemon.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	called := make(map[string]bool)
	closedActivationCalls := 0
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
			if ok {
				called[identifier.Name] = true
			}
			if !ok || identifier.Name != "activateProductComposition" {
				return true
			}
			closedActivationCalls++
			if len(call.Args) != 5 {
				t.Errorf("closed production activation args=%d", len(call.Args))
				return true
			}
			construction, ok := call.Args[4].(*ast.CompositeLit)
			if !ok {
				t.Errorf("production activation construction=%T", call.Args[4])
				return true
			}
			constructionType, ok := construction.Type.(*ast.Ident)
			if !ok || constructionType.Name != "productCompatibilityConstruction" {
				t.Errorf("production activation construction type=%#v", construction.Type)
			}
			for _, element := range construction.Elts {
				field, ok := element.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				key, ok := field.Key.(*ast.Ident)
				if ok && key.Name == "handler" {
					t.Error("production activation passes a direct handler")
				}
			}
			return true
		})
	}
	if closedActivationCalls != 1 || !called["newProductLocalIPCHandlerFactory"] ||
		called["activateProductCompatibilityComposition"] ||
		called["newProductRouteHandler"] || called["localProductHandlerWithComposition"] {
		t.Fatalf("production calls=%v", called)
	}
}

func routeMethods(routes []composition.RouteDescriptor) []composition.RouteMethod {
	result := make([]composition.RouteMethod, len(routes))
	for index, route := range routes {
		result[index] = route.Method
	}
	return result
}
