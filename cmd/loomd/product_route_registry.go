package main

import (
	"context"
	"sort"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/composition"
	"loom-pi-rebuild/internal/credentials"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/production"
)

type productRouteGroup struct {
	owner               string
	handler             composition.CapabilityRef
	availabilityFailure string
	incidentPolicy      string
	privacyClass        composition.PrivacyClass
	methods             []composition.RouteMethod
}

func productRouteManifest() []composition.RouteDescriptor {
	assets := composition.CapabilityAssetsReader.Ref()
	conversation := composition.CapabilityConversationRouter.Ref()
	governance := composition.CapabilityApprovedToolInvoker.Ref()
	runtime := composition.CapabilityRuntimeDispatcher.Ref()
	vault := composition.CapabilityCredentialLeaseIssuer.Ref()
	work := composition.CapabilityWorkCoordinator.Ref()
	groups := []productRouteGroup{
		{
			owner: "loom-assets", handler: assets,
			availabilityFailure: "state_unavailable", incidentPolicy: "preserve_journey",
			privacyClass: composition.PrivacyLocalContent,
			methods: []composition.RouteMethod{
				"evolution_asset_command", "evolution_asset_diff", "evolution_asset_snapshot",
			},
		},
		{
			owner: "loom-conversation", handler: conversation,
			availabilityFailure: "state_unavailable", incidentPolicy: "preserve_request",
			privacyClass: composition.PrivacyLocalContent,
			methods:      []composition.RouteMethod{"chat_message", "chat_thread"},
		},
		{
			owner: "loom-governance", handler: governance,
			availabilityFailure: "state_unavailable", incidentPolicy: "preserve_request",
			privacyClass: composition.PrivacyLocalContent,
			methods: []composition.RouteMethod{
				"customer_rule_command", "customer_rule_snapshot", "mission_decision",
				"permissions_attention", "permissions_command", "permissions_snapshot",
				"provider_account_policy_configure", "provider_model_rate_card_configure",
				"setup_snapshot", "standing_order_command", "standing_order_snapshot",
			},
		},
		{
			owner: "loom-vault", handler: vault,
			availabilityFailure: "state_unavailable", incidentPolicy: "preserve_credential",
			privacyClass: composition.PrivacyLocalContent,
			methods: []composition.RouteMethod{
				"credential_configure", "credential_replace", "credential_revoke", "credential_verify",
			},
		},
		{
			owner: "loom-vault", handler: vault,
			availabilityFailure: "credential_unavailable", incidentPolicy: "preserve_credential",
			privacyClass: composition.PrivacyLocalContent,
			methods: []composition.RouteMethod{
				"credential_vault_export", "credential_vault_lock", "credential_vault_reset",
				"credential_vault_rotate", "credential_vault_unlock",
			},
		},
		{
			owner: "loom-agent-runtime", handler: runtime,
			availabilityFailure: "state_unavailable", incidentPolicy: "preserve_request",
			privacyClass: composition.PrivacyLocalContent,
			methods: []composition.RouteMethod{
				"agent_attempt_recovery", "agent_input", "codex_connect", "mission_execution",
			},
		},
		{
			owner: "loom-agent-runtime", handler: runtime,
			availabilityFailure: "internal", incidentPolicy: "preserve_request",
			privacyClass: composition.PrivacyLocalContent,
			methods: []composition.RouteMethod{
				"side_task_handoff",
			},
		},
		{
			owner: "loom-work", handler: work,
			availabilityFailure: "state_unavailable", incidentPolicy: "preserve_journey",
			privacyClass: composition.PrivacyLocalContent,
			methods: []composition.RouteMethod{
				"builder_answer", "builder_confirm", "builder_edit", "builder_start",
				"builder_validate", "execution_command", "execution_snapshot",
				"integration_command", "integration_snapshot", "production_command",
				"production_snapshot", "queue_command", "queue_snapshot", "snapshot",
				"team_archive", "team_restore", "timeline_page", "workers_command",
				"workers_snapshot", "tool_recovery",
			},
		},
	}
	routes := make([]composition.RouteDescriptor, 0, 48)
	for _, group := range groups {
		for _, method := range group.methods {
			routes = append(routes, composition.RouteDescriptor{
				Method: method, SchemaVersion: 1, OwnerBundle: group.owner,
				RequiredCapabilities: []composition.CapabilityRef{group.handler},
				HandlerCapability:    group.handler,
				AvailabilityFailure:  group.availabilityFailure,
				IncidentPolicy:       group.incidentPolicy, PrivacyClass: group.privacyClass,
			})
		}
	}
	sort.Slice(routes, func(left, right int) bool {
		return routes[left].Method < routes[right].Method
	})
	return routes
}

func productRouteMethods(routes []composition.RouteDescriptor) []composition.RouteMethod {
	result := make([]composition.RouteMethod, len(routes))
	for index, route := range routes {
		result[index] = route.Method
	}
	return result
}

type productRouteAdmission func(context.Context, localipc.Request) (localipc.Response, bool)

type productRegisteredRoute struct {
	descriptor composition.RouteDescriptor
	admit      productRouteAdmission
}

type productRouteRegistry struct {
	routes map[composition.RouteMethod]productRegisteredRoute
}

func newProductRouteRegistry(services productRouteServices) productRouteRegistry {
	manifest := productRouteManifest()
	registry := productRouteRegistry{
		routes: make(map[composition.RouteMethod]productRegisteredRoute, len(manifest)),
	}
	for _, descriptor := range manifest {
		registry.routes[descriptor.Method] = productRegisteredRoute{
			descriptor: descriptor,
			admit:      productRouteAdmissionFor(descriptor.Method, services),
		}
	}
	return registry
}

func (registry productRouteRegistry) admit(
	ctx context.Context,
	request localipc.Request,
) (localipc.Response, bool) {
	route, found := registry.routes[composition.RouteMethod(request.Method)]
	if !found {
		return localipc.Response{}, false
	}
	return route.admit(ctx, request)
}

func productRouteAdmissionFor(
	method composition.RouteMethod,
	services productRouteServices,
) productRouteAdmission {
	return func(ctx context.Context, request localipc.Request) (localipc.Response, bool) {
		reject := func(response localipc.Response) (localipc.Response, bool) {
			return response, true
		}
		available := func() (localipc.Response, bool) {
			return localipc.Response{}, false
		}
		switch {
		case productSetupMethod(string(method)):
			if nilProductAssetPort(services.setup) {
				return reject(productErrorResponse(
					"state_unavailable", api.ErrInvalidLocalProductSetupAPI,
				))
			}
		case productCredentialVaultMethod(method):
			if services.credentialVault == nil {
				return reject(productCredentialErrorResponse(
					"credential_unavailable", credentials.ErrCredentialStoreUnavailable,
				))
			}
		case method == "mission_decision":
			if services.decision == nil {
				return reject(productErrorResponse(
					"state_unavailable", api.ErrInvalidLocalProductDecisionAPI,
				))
			}
		case method == "mission_execution":
			if services.missionExecution == nil {
				return reject(productErrorResponse(
					"state_unavailable", api.ErrInvalidLocalProductExecutionAPI,
				))
			}
		case method == "agent_input":
			if services.agentInput == nil {
				return reject(productAgentInputErrorResponse(
					"state_unavailable", errProductInvalidAgentInput,
				))
			}
		case method == "agent_attempt_recovery":
			if services.agentRecovery == nil {
				return reject(productAgentAttemptRecoveryErrorResponse(
					"state_unavailable", errProductInvalidAttemptRecoveryRequest,
				))
			}
		case method == "tool_recovery":
			if services.toolRecovery == nil {
				return reject(productToolRecoveryErrorResponse(
					"state_unavailable", errProductInvalidToolRecoveryRequest,
				))
			}
		case method == "side_task_handoff":
			if services.handoff == nil {
				return reject(productErrorResponse(
					"internal", api.ErrInvalidLocalProductHandoffAPI,
				))
			}
		case method == "snapshot" || method == "timeline_page" ||
			method == "chat_thread" || method == "chat_message":
			if nilProductAssetPort(services.read) {
				response := productErrorResponse(
					"state_unavailable", api.ErrLocalProductStateUnavailable,
				)
				if method == "chat_message" {
					response = productConversationResponseStage(response, "conversation_dispatch")
				}
				return reject(response)
			}
		case productAssetMethod(string(method)):
			if services.assets == nil {
				return reject(productJourneyErrorResponse(
					request.JourneyID, "state_unavailable", api.ErrInvalidLocalProductAssetAPI,
				))
			}
		case productQueueMethod(string(method)):
			if services.queue == nil {
				return reject(productJourneyErrorResponse(
					request.JourneyID, "state_unavailable", api.ErrInvalidLocalQueueAPI,
				))
			}
		case productWorkersMethod(string(method)):
			if services.workers == nil {
				return reject(productJourneyErrorResponse(
					request.JourneyID, "state_unavailable", api.ErrInvalidLocalQueueAPI,
				))
			}
		case productIntegrationMethod(string(method)):
			if services.integration == nil {
				return reject(productJourneyErrorResponse(
					request.JourneyID, "state_unavailable", api.ErrInvalidLocalQueueAPI,
				))
			}
		case productPermissionMethod(string(method)):
			if services.permission == nil {
				return reject(productJourneyErrorResponse(
					request.JourneyID, "state_unavailable", app.ErrPermissionStateUnavailable,
				))
			}
		case productCustomerRuleMethod(string(method)):
			if services.customerRule == nil {
				return reject(productJourneyErrorResponse(
					request.JourneyID, "state_unavailable", app.ErrInvalidCustomerRuleRequest,
				))
			}
		case productStandingOrderMethod(string(method)):
			if services.standingOrder == nil {
				return reject(productJourneyErrorResponse(
					request.JourneyID, "state_unavailable", app.ErrInvalidStandingOrderRequest,
				))
			}
		case productExecutionMethod(string(method)):
			if services.execution == nil {
				return reject(productJourneyErrorResponse(
					request.JourneyID, "state_unavailable", app.ErrExecutionUnavailable,
				))
			}
		case productProductionMethod(string(method)):
			if services.production == nil {
				return reject(productJourneyErrorResponse(
					request.JourneyID, "state_unavailable", app.ErrProductionUnavailable,
				))
			}
		}
		if services.production != nil && services.production.Degraded(ctx) &&
			productionWriteMethod(string(method)) {
			return reject(productJourneyErrorResponse(
				request.JourneyID, "degraded", production.ErrDegraded,
			))
		}
		return available()
	}
}

func productCredentialVaultMethod(method composition.RouteMethod) bool {
	switch method {
	case "credential_vault_export", "credential_vault_lock", "credential_vault_reset",
		"credential_vault_rotate", "credential_vault_unlock":
		return true
	default:
		return false
	}
}
