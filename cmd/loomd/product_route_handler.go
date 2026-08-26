package main

import (
	"context"
	"errors"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/credentials"
	credentialvault "loom-pi-rebuild/internal/credentials/vault"
	"loom-pi-rebuild/internal/harnessgateway"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/provider/failurelab"
	"loom-pi-rebuild/internal/roundtable"
)

type productRouteServices struct {
	read                  productReadRoute
	chatCancel            api.LocalProductConversationResponseCanceller
	setup                 productSetupRoute
	decision              productDecisionRoute
	missionExecution      productMissionExecutionRoute
	roundtable            productRoundtableRoute
	agentRecovery         productAgentAttemptRecoveryRoute
	toolRecovery          productToolRecoveryRoute
	agentInput            productAgentInputRoute
	handoff               productHandoffRoute
	savedTeamMaterializer productSavedTeamMaterializer
	assets                productAssetRoute
	queue                 productQueueRoute
	workers               productWorkersRoute
	integration           productIntegrationRoute
	permission            productPermissionRoute
	execution             productExecutionRoute
	production            productProductionRoute
	customerRule          productCustomerRuleRoute
	standingOrder         productStandingOrderRoute
	credentialVault       productCredentialVaultController
	failureLab            *failurelab.Runner
}

type productChatMessageRouteRequest struct {
	ThreadID                    string                                        `json:"thread_id"`
	Content                     string                                        `json:"content"`
	ProfileID                   string                                        `json:"profile_id"`
	ModelID                     string                                        `json:"model_id,omitempty"`
	ReasoningEffort             string                                        `json:"reasoning_effort,omitempty"`
	ContextMode                 api.LocalProductContextMode                   `json:"context_mode"`
	ExpectedExecutionBinding    *api.LocalProductConversationExecutionBinding `json:"expected_execution_binding,omitempty"`
	TrustBoundaryAcknowledgment *api.LocalProductTrustBoundaryAcknowledgement `json:"trust_boundary_acknowledgement,omitempty"`
}

func (request productChatMessageRouteRequest) chatMessageRequest(
	incidentID string,
) api.LocalProductChatMessageRequest {
	return api.LocalProductChatMessageRequest{
		ThreadID: request.ThreadID, Content: request.Content,
		ProfileID: request.ProfileID, ModelID: request.ModelID,
		ReasoningEffort: request.ReasoningEffort, ContextMode: request.ContextMode,
		ExpectedExecutionBinding:     request.ExpectedExecutionBinding,
		TrustBoundaryAcknowledgement: request.TrustBoundaryAcknowledgment,
		IncidentID:                   incidentID,
	}
}

func newProductRouteHandler(
	services productRouteServices,
) func(context.Context, localipc.Request) localipc.Response {
	service := services.read
	chatCanceller := services.chatCancel
	setup := services.setup
	decision := services.decision
	execution := services.missionExecution
	agentRecovery := services.agentRecovery
	toolRecovery := services.toolRecovery
	agentInput := services.agentInput
	handoff := services.handoff
	savedTeamMaterializer := services.savedTeamMaterializer
	assetService := services.assets
	queueService := services.queue
	workersService := services.workers
	integrationService := services.integration
	permissionService := services.permission
	executionService := services.execution
	productionService := services.production
	customerRuleService := services.customerRule
	standingOrderService := services.standingOrder
	credentialVaultController := services.credentialVault
	failureLabRunner := services.failureLab
	roundtableService := services.roundtable
	registry := newProductRouteRegistry(services)
	return func(
		ctx context.Context,
		request localipc.Request,
	) localipc.Response {
		if response, rejected := registry.admit(ctx, request); rejected {
			return response
		}
		switch request.Method {
		case "credential_vault_rotate":
			if decodeExactProductParams(request.Params, &struct{}{}) != nil {
				return productCredentialErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			if err := credentialVaultController.RotateCredentialVault(ctx); err != nil {
				return productCredentialServiceError(err)
			}
			return productResultResponse(struct{}{})
		case "credential_vault_lock":
			if decodeExactProductParams(request.Params, &struct{}{}) != nil {
				return productCredentialErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			if err := credentialVaultController.LockCredentialVault(ctx); err != nil {
				return productCredentialServiceError(err)
			}
			return productResultResponse(struct{}{})
		case "credential_vault_unlock":
			if decodeExactProductParams(request.Params, &struct{}{}) != nil {
				return productCredentialErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			if err := credentialVaultController.UnlockCredentialVault(ctx); err != nil {
				return productCredentialServiceError(err)
			}
			return productResultResponse(struct{}{})
		case "credential_vault_reset":
			var input struct {
				Confirmation string `json:"confirmation"`
			}
			if decodeExactProductParams(request.Params, &input) != nil ||
				input.Confirmation != credentialVaultResetConfirmation {
				return productCredentialErrorResponse(
					"denied",
					credentials.WithCredentialFailureStage(
						credentials.CredentialStageVaultRecovery,
						credentials.ErrCredentialStoreDenied,
					),
				)
			}
			if err := credentialVaultController.ResetCredentialVault(
				ctx, input.Confirmation,
			); err != nil {
				return productCredentialServiceError(err)
			}
			return productResultResponse(struct{}{})
		case "credential_vault_export":
			var input struct {
				Passphrase  []byte `json:"passphrase"`
				Destination string `json:"destination"`
			}
			if decodeExactProductParams(request.Params, &input) != nil ||
				!credentialvault.ValidEncryptedBackupRequest(
					input.Passphrase, input.Destination,
				) {
				clearProductCredentialLeaseSecret(input.Passphrase)
				return productCredentialErrorResponse(
					"invalid_request", api.ErrInvalidLocalProductSetupAPI,
				)
			}
			result, err := credentialVaultController.ExportCredentialVault(
				ctx, input.Passphrase, input.Destination,
			)
			if err != nil {
				return productCredentialServiceError(err)
			}
			return productResultResponse(result)
		case "provider_failure_lab_run":
			var input productFailureLabRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse("invalid_request", errProductFailureLabUnavailable)
			}
			result, err := runProductFailureLab(ctx, failureLabRunner, request.RequestID, input)
			if err != nil {
				return productErrorResponse("invalid_request", err)
			}
			return productResultResponse(result)
		case "snapshot":
			if service == nil {
				return productErrorResponse(
					"state_unavailable",
					api.ErrLocalProductStateUnavailable,
				)
			}
			var input api.LocalProductSnapshotRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductRequest,
				)
			}
			result, err := service.ReadLocalProductSnapshot(ctx, input)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "timeline_page":
			if service == nil {
				return productErrorResponse(
					"state_unavailable",
					api.ErrLocalProductStateUnavailable,
				)
			}
			var input api.LocalProductTimelineRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductRequest,
				)
			}
			result, err := service.ReadLocalProductTimeline(ctx, input)
			if err != nil {
				var gapErr *api.TimelineGapError
				if !errors.As(err, &gapErr) {
					return productServiceError(err)
				}
			}
			return productResultResponse(result)
		case "chat_thread":
			if service == nil {
				return productErrorResponse(
					"state_unavailable",
					api.ErrLocalProductStateUnavailable,
				)
			}
			var input api.LocalProductChatThreadRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductChatRequest,
				)
			}
			result, err := service.ReadChatThread(ctx, input.ThreadID)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "chat_context_disclosure":
			var input api.LocalProductChatContextDisclosureRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductChatRequest,
				)
			}
			result, err := service.ReadChatContextDisclosure(ctx, input)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "chat_thread_delete":
			if service == nil {
				return productErrorResponse(
					"state_unavailable",
					api.ErrLocalProductStateUnavailable,
				)
			}
			var input api.LocalProductChatThreadRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductChatRequest,
				)
			}
			if err := service.DeleteChatThread(ctx, input.ThreadID); err != nil {
				return productServiceError(err)
			}
			return productResultResponse(struct {
				ThreadID string `json:"thread_id"`
				Deleted  bool   `json:"deleted"`
			}{ThreadID: input.ThreadID, Deleted: true})
		case "chat_response_cancel":
			var input api.LocalProductChatResponseCancelRequest
			if decodeExactProductParams(request.Params, &input) != nil ||
				input.ThreadID == "" || input.IncidentID == "" {
				return productErrorResponse(
					"invalid_request", api.ErrInvalidLocalProductChatRequest,
				)
			}
			cancelContext, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := chatCanceller.CancelChatResponse(cancelContext, input)
			cancel()
			if err != nil {
				if errors.Is(err, harnessgateway.ErrResponseNotFound) {
					return productErrorResponse("not_found", err)
				}
				return productServiceError(err)
			}
			return productResultResponse(struct {
				ThreadID   string `json:"thread_id"`
				IncidentID string `json:"incident_id"`
				Cancelled  bool   `json:"cancelled"`
			}{input.ThreadID, input.IncidentID, true})
		case "chat_message":
			if service == nil {
				return productConversationResponseStage(productErrorResponse(
					"state_unavailable",
					api.ErrLocalProductStateUnavailable,
				), "conversation_dispatch")
			}
			var routeInput productChatMessageRouteRequest
			if decodeExactProductParams(request.Params, &routeInput) != nil {
				return productConversationResponseStage(productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductChatRequest,
				), "input_admission")
			}
			input := routeInput.chatMessageRequest(request.RequestID)
			result, err := service.SendChatMessage(ctx, input)
			if err != nil {
				return productConversationServiceError(err)
			}
			return productResultResponse(result)
		case "evolution_asset_snapshot":
			var input api.EvolutionAssetSnapshotRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productJourneyErrorResponse(request.JourneyID, "invalid_request", app.ErrInvalidLocalProductAsset)
			}
			input.JourneyID = request.JourneyID
			result, err := assetService.EvolutionAssetSnapshot(ctx, input)
			if err != nil {
				return productJourneyServiceError(request.JourneyID, err)
			}
			return productJourneyResultResponse(request.JourneyID, result)
		case "evolution_asset_diff":
			var input api.EvolutionAssetDiffRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productJourneyErrorResponse(request.JourneyID, "invalid_request", app.ErrInvalidLocalProductAsset)
			}
			input.JourneyID = request.JourneyID
			result, err := assetService.EvolutionAssetDiff(ctx, input)
			if err != nil {
				return productJourneyServiceError(request.JourneyID, err)
			}
			return productJourneyResultResponse(request.JourneyID, result)
		case "evolution_asset_command":
			var input api.EvolutionAssetCommandRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productJourneyErrorResponse(request.JourneyID, "invalid_request", app.ErrInvalidLocalProductAsset)
			}
			input.JourneyID = request.JourneyID
			result, err := assetService.EvolutionAssetCommand(ctx, input)
			if err != nil {
				return productJourneyServiceError(request.JourneyID, err)
			}
			return productJourneyResultResponse(request.JourneyID, result)
		case "queue_snapshot":
			var input api.QueueSnapshotRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productJourneyErrorResponse(request.JourneyID, "invalid_request", app.ErrInvalidQueueRequest)
			}
			input.JourneyID = request.JourneyID
			result, err := queueService.QueueSnapshot(ctx, input)
			if err != nil {
				return productJourneyServiceError(request.JourneyID, err)
			}
			return productJourneyResultResponse(request.JourneyID, result)
		case "queue_command":
			var input api.QueueCommandRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productJourneyErrorResponse(request.JourneyID, "invalid_request", app.ErrInvalidQueueRequest)
			}
			input.JourneyID = request.JourneyID
			result, err := queueService.QueueCommand(ctx, input)
			if err != nil {
				return productJourneyServiceError(request.JourneyID, err)
			}
			return productJourneyResultResponse(request.JourneyID, result)
		case "workers_snapshot":
			var input app.WorkersSnapshotRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productJourneyErrorResponse(request.JourneyID, "invalid_request", app.ErrInvalidWorkerRequest)
			}
			input.JourneyID = request.JourneyID
			result, err := workersService.WorkersSnapshot(ctx, input)
			if err != nil {
				return productJourneyServiceError(request.JourneyID, err)
			}
			return productJourneyResultResponse(request.JourneyID, result)
		case "workers_command":
			var input app.WorkersCommandRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productJourneyErrorResponse(request.JourneyID, "invalid_request", app.ErrInvalidWorkerRequest)
			}
			input.JourneyID = request.JourneyID
			result, err := workersService.WorkersCommand(ctx, input)
			if err != nil {
				return productJourneyServiceError(request.JourneyID, err)
			}
			return productJourneyResultResponse(request.JourneyID, result)
		case "integration_snapshot":
			result, err := integrationService.Snapshot(ctx)
			if err != nil {
				return productJourneyServiceError(request.JourneyID, err)
			}
			return productJourneyResultResponse(request.JourneyID, result)
		case "integration_command":
			var input app.IntegrationCommandRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productJourneyErrorResponse(request.JourneyID, "invalid_request", app.ErrInvalidIntegrationRequest)
			}
			input.JourneyID = request.JourneyID
			result, err := integrationService.Command(ctx, input)
			if err != nil {
				return productJourneyServiceError(request.JourneyID, err)
			}
			return productJourneyResultResponse(request.JourneyID, result)
		case "customer_rule_snapshot":
			var input app.CustomerRuleSnapshotRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productJourneyErrorResponse(request.JourneyID, "invalid_request", app.ErrInvalidCustomerRuleRequest)
			}
			input.JourneyID = request.JourneyID
			result, err := customerRuleService.Snapshot(ctx, input)
			if err != nil {
				return productJourneyServiceError(request.JourneyID, err)
			}
			return productJourneyResultResponse(request.JourneyID, result)
		case "customer_rule_command":
			var input app.CustomerRuleCommandRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productJourneyErrorResponse(request.JourneyID, "invalid_request", app.ErrInvalidCustomerRuleRequest)
			}
			input.JourneyID = request.JourneyID
			result, err := customerRuleService.Command(ctx, input)
			if err != nil {
				return productJourneyServiceError(request.JourneyID, err)
			}
			return productJourneyResultResponse(request.JourneyID, result)
		case "standing_order_snapshot":
			var input app.StandingOrderSnapshotRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productJourneyErrorResponse(request.JourneyID, "invalid_request", app.ErrInvalidStandingOrderRequest)
			}
			input.JourneyID = request.JourneyID
			result, err := standingOrderService.Snapshot(ctx, input)
			if err != nil {
				return productJourneyServiceError(request.JourneyID, err)
			}
			return productJourneyResultResponse(request.JourneyID, result)
		case "standing_order_command":
			var input app.StandingOrderCommandRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productJourneyErrorResponse(request.JourneyID, "invalid_request", app.ErrInvalidStandingOrderRequest)
			}
			input.JourneyID = request.JourneyID
			result, err := standingOrderService.Command(ctx, input)
			if err != nil {
				return productJourneyServiceError(request.JourneyID, err)
			}
			return productJourneyResultResponse(request.JourneyID, result)
		case "permissions_snapshot":
			var input app.PermissionSnapshotRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productJourneyErrorResponse(request.JourneyID, "invalid_request", app.ErrInvalidPermissionRequest)
			}
			input.JourneyID = request.JourneyID
			result, err := permissionService.PermissionSnapshot(ctx, input)
			if err != nil {
				return productJourneyServiceError(request.JourneyID, err)
			}
			return productJourneyResultResponse(request.JourneyID, result)
		case "permissions_attention":
			var input app.PermissionAttentionRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productJourneyErrorResponse(request.JourneyID, "invalid_request", app.ErrInvalidPermissionRequest)
			}
			input.JourneyID = request.JourneyID
			result, err := permissionService.PermissionAttention(ctx, input)
			if err != nil {
				return productJourneyServiceError(request.JourneyID, err)
			}
			return productJourneyResultResponse(request.JourneyID, result)
		case "permissions_command":
			var input app.PermissionCommandRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productJourneyErrorResponse(request.JourneyID, "invalid_request", app.ErrInvalidPermissionRequest)
			}
			input.JourneyID = request.JourneyID
			result, err := permissionService.PermissionCommand(ctx, input)
			if err != nil {
				return productJourneyServiceError(request.JourneyID, err)
			}
			return productJourneyResultResponse(request.JourneyID, result)
		case "execution_snapshot":
			var input app.ExecutionSnapshotRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productJourneyErrorResponse(request.JourneyID, "invalid_request", app.ErrInvalidExecutionRequest)
			}
			input.JourneyID = request.JourneyID
			result, err := executionService.ExecutionSnapshot(ctx, input)
			if err != nil {
				return productJourneyServiceError(request.JourneyID, err)
			}
			return productJourneyResultResponse(request.JourneyID, result)
		case "execution_command":
			var input app.ExecutionCommandRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productJourneyErrorResponse(request.JourneyID, "invalid_request", app.ErrInvalidExecutionRequest)
			}
			input.JourneyID = request.JourneyID
			result, err := executionService.ExecutionCommand(ctx, input)
			if err != nil {
				return productJourneyServiceError(request.JourneyID, err)
			}
			return productJourneyResultResponse(request.JourneyID, result)
		case "production_snapshot":
			var input app.ProductionSnapshotRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productJourneyErrorResponse(request.JourneyID, "invalid_request", app.ErrInvalidProductionRequest)
			}
			input.JourneyID = request.JourneyID
			result, err := productionService.ProductionSnapshot(ctx, input)
			if err != nil {
				return productJourneyServiceError(request.JourneyID, err)
			}
			return productJourneyResultResponse(request.JourneyID, result)
		case "production_command":
			var input app.ProductionCommandRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productJourneyErrorResponse(request.JourneyID, "invalid_request", app.ErrInvalidProductionRequest)
			}
			input.JourneyID = request.JourneyID
			result, err := productionService.ProductionCommand(ctx, input)
			if err != nil {
				return productJourneyServiceError(request.JourneyID, err)
			}
			return productJourneyResultResponse(request.JourneyID, result)
		case "mission_decision":
			var input app.MissionDecisionCommand
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					app.ErrInvalidMissionDecision,
				)
			}
			if input.Operation == "read" {
				result, err := decision.ReadMissionDecision(ctx, input)
				if err != nil {
					return productServiceError(err)
				}
				return productResultResponse(result)
			}
			result, err := decision.DecideMission(ctx, input)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "mission_execution":
			var input app.MissionExecutionCommand
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					app.ErrInvalidMissionExecution,
				)
			}
			result, err := execution.ExecuteMission(ctx, input)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "roundtable_session_create":
			var input productRoundtableSessionCreateParams
			if decodeExactProductParams(request.Params, &input) != nil ||
				!validProductRoundtableSchemaVersion(input.SchemaVersion) {
				return productErrorResponse("invalid_request", roundtable.ErrInvalidRoundtableSession)
			}
			view, err := roundtableService.CreateSession(ctx, roundtable.CreateSessionCommand{
				SessionID: input.SessionID, ModeratorSeat: input.ModeratorSeat,
				Title: input.Title, CorrelationID: input.CorrelationID,
			})
			if err != nil {
				return productRoundtableServiceError(err)
			}
			return productResultResponse(view)
		case "roundtable_add_seat":
			var input productRoundtableAddSeatParams
			if decodeExactProductParams(request.Params, &input) != nil ||
				!validProductRoundtableSchemaVersion(input.SchemaVersion) {
				return productErrorResponse("invalid_request", roundtable.ErrInvalidRoundtableSeat)
			}
			view, err := roundtableService.AddSeat(ctx, roundtable.AddSeatCommand{
				SessionID: input.SessionID, SeatID: input.SeatID,
				DisplayName: input.DisplayName, CorrelationID: input.CorrelationID,
			})
			if err != nil {
				return productRoundtableServiceError(err)
			}
			return productResultResponse(view)
		case "roundtable_retire_seat":
			var input productRoundtableRetireSeatParams
			if decodeExactProductParams(request.Params, &input) != nil ||
				!validProductRoundtableSchemaVersion(input.SchemaVersion) {
				return productErrorResponse("invalid_request", roundtable.ErrInvalidRoundtableSeat)
			}
			view, err := roundtableService.RetireSeat(ctx, roundtable.RetireSeatCommand{
				SessionID: input.SessionID, SeatID: input.SeatID,
				ModeratorSeat: input.ModeratorSeat, CorrelationID: input.CorrelationID,
			})
			if err != nil {
				return productRoundtableServiceError(err)
			}
			return productResultResponse(view)
		case "roundtable_open_round":
			var input productRoundtableOpenRoundParams
			if decodeExactProductParams(request.Params, &input) != nil ||
				!validProductRoundtableSchemaVersion(input.SchemaVersion) {
				return productErrorResponse("invalid_request", roundtable.ErrInvalidRoundtableMessage)
			}
			view, err := roundtableService.OpenRound(ctx, roundtable.OpenRoundCommand{
				SessionID: input.SessionID, RoundID: input.RoundID,
				ModeratorSeat: input.ModeratorSeat, CorrelationID: input.CorrelationID,
			})
			if err != nil {
				return productRoundtableServiceError(err)
			}
			return productResultResponse(view)
		case "roundtable_propose_message":
			var input productRoundtableProposeMessageParams
			if decodeExactProductParams(request.Params, &input) != nil ||
				!validProductRoundtableSchemaVersion(input.SchemaVersion) {
				return productErrorResponse("invalid_request", roundtable.ErrInvalidRoundtableMessage)
			}
			view, err := roundtableService.ProposeMessage(ctx, roundtable.ProposeMessageCommand{
				SessionID: input.SessionID, RoundID: input.RoundID,
				MessageID: input.MessageID, WriterSeat: input.WriterSeat,
				TargetSeat: input.TargetSeat, Body: input.Body,
				ArtifactRefs: input.ArtifactRefs, CorrelationID: input.CorrelationID,
			})
			if err != nil {
				return productRoundtableServiceError(err)
			}
			return productResultResponse(view)
		case "roundtable_relay_message":
			var input productRoundtableRelayMessageParams
			if decodeExactProductParams(request.Params, &input) != nil ||
				!validProductRoundtableSchemaVersion(input.SchemaVersion) {
				return productErrorResponse("invalid_request", roundtable.ErrInvalidRoundtableMessage)
			}
			view, err := roundtableService.RelayMessage(ctx, roundtable.RelayMessageCommand{
				SessionID: input.SessionID, MessageID: input.MessageID,
				ModeratorSeat: input.ModeratorSeat, CorrelationID: input.CorrelationID,
			})
			if err != nil {
				return productRoundtableServiceError(err)
			}
			return productResultResponse(view)
		case "roundtable_ack_message":
			var input productRoundtableAckMessageParams
			if decodeExactProductParams(request.Params, &input) != nil ||
				!validProductRoundtableSchemaVersion(input.SchemaVersion) {
				return productErrorResponse("invalid_request", roundtable.ErrInvalidRoundtableMessage)
			}
			view, err := roundtableService.AcknowledgeMessage(ctx, roundtable.AcknowledgeMessageCommand{
				SessionID: input.SessionID, MessageID: input.MessageID,
				SeatID: input.SeatID, CorrelationID: input.CorrelationID,
			})
			if err != nil {
				return productRoundtableServiceError(err)
			}
			return productResultResponse(view)
		case "roundtable_insert_message":
			var input productRoundtableInsertMessageParams
			if decodeExactProductParams(request.Params, &input) != nil ||
				!validProductRoundtableSchemaVersion(input.SchemaVersion) {
				return productErrorResponse("invalid_request", roundtable.ErrInvalidRoundtableMessage)
			}
			view, err := roundtableService.InsertMessage(ctx, roundtable.InsertMessageCommand{
				SessionID: input.SessionID, MessageID: input.MessageID,
				ModeratorSeat: input.ModeratorSeat, CorrelationID: input.CorrelationID,
			})
			if err != nil {
				return productRoundtableServiceError(err)
			}
			return productResultResponse(view)
		case "roundtable_drop_message":
			var input productRoundtableDropMessageParams
			if decodeExactProductParams(request.Params, &input) != nil ||
				!validProductRoundtableSchemaVersion(input.SchemaVersion) {
				return productErrorResponse("invalid_request", roundtable.ErrInvalidRoundtableMessage)
			}
			view, err := roundtableService.DropMessage(ctx, roundtable.DropMessageCommand{
				SessionID: input.SessionID, MessageID: input.MessageID,
				ModeratorSeat: input.ModeratorSeat, CorrelationID: input.CorrelationID,
			})
			if err != nil {
				return productRoundtableServiceError(err)
			}
			return productResultResponse(view)
		case "roundtable_conclude":
			var input productRoundtableConcludeParams
			if decodeExactProductParams(request.Params, &input) != nil ||
				!validProductRoundtableSchemaVersion(input.SchemaVersion) {
				return productErrorResponse("invalid_request", roundtable.ErrInvalidRoundtableSession)
			}
			view, err := roundtableService.ConcludeSession(ctx, roundtable.ConcludeSessionCommand{
				SessionID: input.SessionID, ModeratorSeat: input.ModeratorSeat,
				CorrelationID: input.CorrelationID,
			})
			if err != nil {
				return productRoundtableServiceError(err)
			}
			return productResultResponse(view)
		case "roundtable_snapshot":
			var input productRoundtableSnapshotParams
			if decodeExactProductParams(request.Params, &input) != nil ||
				!validProductRoundtableSchemaVersion(input.SchemaVersion) {
				return productErrorResponse("invalid_request", roundtable.ErrInvalidRoundtableSession)
			}
			view, err := roundtableService.ReadView(ctx, input.SessionID)
			if err != nil {
				return productRoundtableServiceError(err)
			}
			return productResultResponse(view)
		case "agent_attempt_recovery":
			var input productAgentAttemptRecoveryRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productAgentAttemptRecoveryErrorResponse(
					"invalid_request", errProductInvalidAttemptRecoveryRequest,
				)
			}
			input.IncidentID = request.RequestID
			result, err := agentRecovery.RecoverAgentAttempt(ctx, input)
			if err != nil {
				return productAgentAttemptRecoveryServiceError(err)
			}
			return productResultResponse(result)
		case "tool_recovery":
			var input productToolRecoveryRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				return productToolRecoveryErrorResponse(
					"invalid_request", errProductInvalidToolRecoveryRequest,
				)
			}
			input.IncidentID = request.RequestID
			result, err := toolRecovery.RecoverToolCall(ctx, input)
			if err != nil {
				return productToolRecoveryServiceError(err)
			}
			return productResultResponse(result)
		case "agent_input":
			var input productAgentInputRequest
			if decodeExactProductParams(request.Params, &input) != nil {
				clearProductAgentInput(input.Content)
				return productAgentInputErrorResponse(
					"invalid_request", errProductInvalidAgentInput,
				)
			}
			input.IncidentID = request.RequestID
			result, err := agentInput.AdmitAgentInput(ctx, input)
			if err != nil {
				return productAgentInputServiceError(err)
			}
			return productResultResponse(result)
		case "side_task_handoff":
			operation, err := productOperation(request.Params)
			if err != nil {
				return productErrorResponse("invalid_request", app.ErrInvalidSideTaskProduct)
			}
			switch operation {
			case "propose":
				var input app.SideTaskProposalRequest
				if decodeExactProductParams(request.Params, &input) != nil {
					return productErrorResponse("invalid_request", app.ErrInvalidSideTaskProduct)
				}
				result, err := handoff.ProposeSideTask(ctx, input)
				if err != nil {
					return sideTaskServiceError(err)
				}
				return productResultResponse(result)
			case "create":
				var input app.SideTaskCreateRequest
				if decodeExactProductParams(request.Params, &input) != nil {
					return productErrorResponse("invalid_request", app.ErrInvalidSideTaskProduct)
				}
				result, err := handoff.CreateSideTask(ctx, input)
				if err != nil {
					return sideTaskServiceError(err)
				}
				return productResultResponse(result)
			case "read":
				var input app.SideTaskReadRequest
				if decodeExactProductParams(request.Params, &input) != nil {
					return productErrorResponse("invalid_request", app.ErrInvalidSideTaskProduct)
				}
				result, err := handoff.ReadSideTask(ctx, input)
				if err != nil {
					return sideTaskServiceError(err)
				}
				return productResultResponse(result)
			case "decide":
				var input app.SideTaskDecisionRequest
				if decodeExactProductParams(request.Params, &input) != nil {
					return productErrorResponse("invalid_request", app.ErrInvalidSideTaskProduct)
				}
				result, err := handoff.DecideSideTask(ctx, input)
				if err != nil {
					return sideTaskServiceError(err)
				}
				return productResultResponse(result)
			default:
				return productErrorResponse("invalid_request", app.ErrInvalidSideTaskProduct)
			}
		case "setup_snapshot":
			if decodeExactProductParams(
				request.Params,
				&struct{}{},
			) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			result, err := setup.SetupSnapshot(ctx)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "provider_account_policy_configure":
			var input app.ProviderAccountPolicyCommand
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			input.CorrelationID = request.RequestID
			result, err := setup.ConfigureProviderAccountPolicy(ctx, input)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "provider_model_rate_card_configure":
			var input app.ProviderModelRateCardCommand
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			input.CorrelationID = request.RequestID
			result, err := setup.ConfigureProviderModelRateCard(ctx, input)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "remote_tool_backend_enrollment_configure":
			var input app.RemoteToolBackendEnrollmentCommand
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			input.CorrelationID = request.RequestID
			result, err := setup.ConfigureRemoteToolBackendEnrollment(ctx, input)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "remote_tool_backend_enrollment_revoke":
			var input app.RemoteToolBackendEnrollmentRevokeCommand
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			input.CorrelationID = request.RequestID
			result, err := setup.RevokeRemoteToolBackendEnrollment(ctx, input)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "codex_connect":
			if decodeExactProductParams(
				request.Params,
				&struct{}{},
			) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			result, err := setup.ConnectCodex(ctx)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "builder_start":
			var input app.BuilderStartCommand
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			result, err := setup.StartBuilder(ctx, input)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "builder_answer":
			var input app.BuilderAnswerCommand
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			result, err := setup.AnswerBuilder(ctx, input)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "builder_edit":
			var input app.BuilderEditCommand
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			result, err := setup.EditBuilder(ctx, input)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "builder_validate":
			var input app.BuilderValidateCommand
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			result, err := setup.ValidateBuilder(ctx, input)
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "builder_confirm":
			var input app.BuilderConfirmCommand
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			result, err := setup.ConfirmBuilder(ctx, input)
			if err != nil {
				return productServiceError(err)
			}
			if savedTeamMaterializer != nil {
				result, err = savedTeamMaterializer.MaterializeConfirmedTeam(
					ctx,
					result,
				)
				if err != nil {
					return productServiceError(err)
				}
			}
			return productResultResponse(result)
		case "team_archive", "team_restore":
			var input app.TeamStatusCommand
			if decodeExactProductParams(request.Params, &input) != nil {
				return productErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			var (
				result app.SetupSavedTeamPreview
				err    error
			)
			if request.Method == "team_archive" {
				result, err = setup.ArchiveTeam(ctx, input)
			} else {
				result, err = setup.RestoreTeam(ctx, input)
			}
			if err != nil {
				return productServiceError(err)
			}
			return productResultResponse(result)
		case "credential_configure",
			"credential_verify",
			"credential_replace",
			"credential_revoke":
			var input productCredentialParams
			if decodeExactProductParams(request.Params, &input) != nil {
				return productCredentialErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			if request.Method == "credential_verify" &&
				!validProductCredentialOperationID(input.OperationID) ||
				request.Method != "credential_verify" && input.OperationID != "" {
				return productCredentialErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			secret := []byte(input.Secret)
			defer clearProductSecret(secret)
			command := app.CredentialSetupCommand{
				ProviderID:          input.ProviderID,
				ProviderAccountID:   input.ProviderAccountID,
				CredentialReference: input.CredentialReference,
				ExpectedRevision:    input.ExpectedRevision,
				OperationID:         input.OperationID,
				Secret:              secret,
			}
			var (
				result app.CredentialSetupResult
				err    error
			)
			switch request.Method {
			case "credential_configure":
				result, err = setup.ConfigureCredential(ctx, command)
			case "credential_verify":
				result, err = setup.VerifyCredential(ctx, command)
			case "credential_replace":
				result, err = setup.ReplaceCredential(ctx, command)
			case "credential_revoke":
				result, err = setup.RevokeCredential(ctx, command)
			}
			if err != nil {
				return productCredentialServiceError(err)
			}
			return productResultResponse(result)
		case "credential_import":
			var input app.CredentialImportCommand
			if decodeExactProductParams(request.Params, &input) != nil {
				return productCredentialErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			result, err := setup.ImportCredentialCandidate(ctx, input)
			if err != nil {
				return productCredentialServiceError(err)
			}
			return productResultResponse(result)
		case "provider_endpoint_review_approve":
			var input app.EndpointReviewCommand
			if decodeExactProductParams(request.Params, &input) != nil {
				return productCredentialErrorResponse(
					"invalid_request",
					api.ErrInvalidLocalProductSetupAPI,
				)
			}
			result, err := setup.ApproveEndpointCandidate(ctx, input)
			if err != nil {
				return productCredentialServiceError(err)
			}
			return productResultResponse(result)
		default:
			return productErrorResponse(
				"unknown_method",
				errors.New("unknown method"),
			)
		}
	}
}
