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
		"builder_validate", "chat_message", "chat_thread", "codex_connect",
		"credential_configure", "credential_replace", "credential_revoke",
		"credential_vault_export", "credential_vault_lock", "credential_vault_reset",
		"credential_vault_rotate", "credential_vault_unlock", "credential_verify",
		"customer_rule_command", "customer_rule_snapshot", "evolution_asset_command",
		"evolution_asset_diff", "evolution_asset_snapshot", "execution_command",
		"execution_snapshot", "integration_command", "integration_snapshot",
		"mission_decision", "mission_execution", "permissions_attention",
		"permissions_command", "permissions_snapshot", "production_command",
		"production_snapshot", "provider_account_policy_configure",
		"provider_model_rate_card_configure", "queue_command", "queue_snapshot",
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
		for _, route := range snapshot.Routes {
			if route.Method == productCompatibilityDispatchRoute {
				t.Fatal("snapshot retained aggregate legacy route")
			}
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
	if _, rejected := registry.admit(context.Background(), localipc.Request{
		Method: "unknown_method",
	}); rejected {
		t.Fatal("registry claimed an unknown method")
	}
}

func TestCOMP2BProductStartupCannotCallPositionalCompatibilityHandler(t *testing.T) {
	parsed, err := parser.ParseFile(token.NewFileSet(), "product_daemon.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	called := make(map[string]bool)
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
			return true
		})
	}
	if !called["newProductLocalIPCHandlerFactory"] ||
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
