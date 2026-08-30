package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/controltool"
	"loom-pi-rebuild/internal/roundtable"
)

const (
	productConversationControlMetadataSnapshotLimit = 64
	productConversationControlMetadataDefaultLimit  = 8
	productConversationControlMetadataMaximumLimit  = 20
	productConversationControlMetadataMaximumResult = 56 << 10
	productConversationControlIncidentMaximumEvents = 32
)

var errProductConversationControlMetadataUnavailable = errors.New(
	"product Conversation control metadata unavailable",
)

type productConversationControlReadSource interface {
	ReadLocalProductSnapshot(
		context.Context,
		api.LocalProductSnapshotRequest,
	) (api.LocalProductSnapshot, error)
}

type productConversationControlSetupSource interface {
	SetupSnapshot(context.Context) (app.SetupSnapshot, error)
}

type productConversationControlRoundtableSource interface {
	ReadView(context.Context, string) (roundtable.View, error)
}

type productConversationControlIncidentSource interface {
	IncidentDiagnostics(
		context.Context,
		string,
	) ([]productIncidentDiagnosticEvent, error)
}

// productIncidentDiagnosticEvent is the explicit content-negative incident
// projection available to models. It intentionally cannot carry paths,
// prompts, Provider bodies, headers, credentials, ciphertext or nonces.
type productIncidentDiagnosticEvent struct {
	OccurredAt         string `json:"occurred_at"`
	IncidentID         string `json:"incident_id"`
	Operation          string `json:"operation"`
	ProviderID         string `json:"provider_id,omitempty"`
	ProviderAccountID  string `json:"provider_account_id,omitempty"`
	CredentialRevision int64  `json:"credential_revision,omitempty"`
	ModelID            string `json:"model_id,omitempty"`
	ReasoningEffort    string `json:"reasoning_effort,omitempty"`
	ThreadID           string `json:"thread_id,omitempty"`
	ProfileID          string `json:"profile_id,omitempty"`
	SessionID          string `json:"session_id,omitempty"`
	HarnessID          string `json:"harness_id,omitempty"`
	BackendID          string `json:"backend_id,omitempty"`
	SegmentID          string `json:"segment_id,omitempty"`
	WorkspaceID        string `json:"workspace_id,omitempty"`
	ResponseID         string `json:"response_id,omitempty"`
	WorkItemID         string `json:"work_item_id,omitempty"`
	RunID              string `json:"run_id,omitempty"`
	RuntimeInstanceID  string `json:"runtime_instance_id,omitempty"`
	AgentID            string `json:"agent_id,omitempty"`
	RoleID             string `json:"role_id,omitempty"`
	Tool               string `json:"tool,omitempty"`
	Stage              string `json:"stage"`
	ElapsedMS          int64  `json:"elapsed_ms"`
	Result             string `json:"result"`
	ErrorCode          string `json:"error_code,omitempty"`
	HTTPStatus         int    `json:"http_status,omitempty"`
	ProviderErrorCode  string `json:"provider_error_code,omitempty"`
	RetryAfterSeconds  int64  `json:"retry_after_seconds,omitempty"`
	Retryable          bool   `json:"retryable"`
}

type productConversationControlMetadataSources struct {
	Read       productConversationControlReadSource
	Setup      productConversationControlSetupSource
	Roundtable productConversationControlRoundtableSource
	Incidents  productConversationControlIncidentSource
}

type productConversationControlMetadataGateway struct {
	sources productConversationControlMetadataSources
}

func (gateway *productConversationControlMetadataGateway) ValidateRoundTableActionTarget(
	ctx context.Context,
	toolID controltool.ToolID,
	payload controltool.ConversationActionPayload,
) error {
	if toolID != controltool.ToolRoundtablesPausePreview {
		return controltool.ErrInvalidCall
	}
	view, err := gateway.currentRoundTableActionView(ctx, payload, false)
	if err == nil && view.Rounds[len(view.Rounds)-1].PauseRequested {
		return controltool.ErrInvalidCall
	}
	return err
}

func (gateway *productConversationControlMetadataGateway) ResolveRoundTableActionTarget(
	ctx context.Context,
	toolID controltool.ToolID,
	payload controltool.ConversationActionPayload,
) (api.LocalProductConversationRoundTableTargetBinding, error) {
	switch toolID {
	case controltool.ToolRoundtablesSteerPreview,
		controltool.ToolRoundtablesRetryPreview,
		controltool.ToolRoundtablesSkipPreview,
		controltool.ToolRoundtablesReplacePreview:
	default:
		return api.LocalProductConversationRoundTableTargetBinding{}, controltool.ErrInvalidCall
	}
	view, err := gateway.currentRoundTableActionView(ctx, payload, true)
	if err != nil {
		return api.LocalProductConversationRoundTableTargetBinding{}, err
	}
	seat := view.Seats[payload.SeatID]
	if toolID == controltool.ToolRoundtablesSkipPreview ||
		toolID == controltool.ToolRoundtablesReplacePreview {
		for _, attempt := range view.Attempts {
			if attempt.RoundID == payload.RoundID && attempt.SeatID == payload.SeatID &&
				attempt.Status == roundtable.SeatAttemptRunning {
				return api.LocalProductConversationRoundTableTargetBinding{}, controltool.ErrInvalidCall
			}
		}
		for _, intervention := range view.Interventions {
			if intervention.Kind == roundtable.InterventionSkipSeat &&
				intervention.RoundID == payload.RoundID && intervention.SeatID == payload.SeatID {
				return api.LocalProductConversationRoundTableTargetBinding{}, controltool.ErrInvalidCall
			}
		}
	}
	if toolID == controltool.ToolRoundtablesSteerPreview ||
		toolID == controltool.ToolRoundtablesRetryPreview {
		attempt, found := view.Attempts[payload.AttemptID]
		if !found || attempt.RoundID != payload.RoundID ||
			attempt.SeatID != payload.SeatID ||
			attempt.MembershipRevision != seat.Binding.MembershipRevision ||
			attempt.SeatBindingDigest != seat.Binding.BindingDigest {
			return api.LocalProductConversationRoundTableTargetBinding{}, controltool.ErrInvalidCall
		}
		if toolID == controltool.ToolRoundtablesSteerPreview &&
			attempt.Status != roundtable.SeatAttemptRunning {
			return api.LocalProductConversationRoundTableTargetBinding{}, controltool.ErrInvalidCall
		}
		if toolID == controltool.ToolRoundtablesRetryPreview &&
			attempt.Status != roundtable.SeatAttemptCancelled &&
			(attempt.Status != roundtable.SeatAttemptFailed || !attempt.Retryable) {
			return api.LocalProductConversationRoundTableTargetBinding{}, controltool.ErrInvalidCall
		}
	}
	target := api.LocalProductConversationRoundTableTargetBinding{
		MembershipRevision: seat.Binding.MembershipRevision,
		SeatBindingDigest:  seat.Binding.BindingDigest,
	}
	if !target.Valid() {
		return api.LocalProductConversationRoundTableTargetBinding{}, controltool.ErrInvalidCall
	}
	return target, nil
}

func (gateway *productConversationControlMetadataGateway) currentRoundTableActionView(
	ctx context.Context,
	payload controltool.ConversationActionPayload,
	requireSeat bool,
) (roundtable.View, error) {
	if gateway == nil || ctx == nil || ctx.Err() != nil ||
		nilProductAssetPort(gateway.sources.Roundtable) ||
		!validProductConversationControlOpaque(payload.SessionID, 128, false) ||
		!validProductConversationControlOpaque(payload.RoundID, 128, false) ||
		requireSeat && !validProductConversationControlOpaque(payload.SeatID, 128, false) {
		return roundtable.View{}, controltool.ErrInvalidCall
	}
	view, err := gateway.sources.Roundtable.ReadView(ctx, payload.SessionID)
	if errors.Is(err, roundtable.ErrRoundtableSessionNotFound) {
		return roundtable.View{}, controltool.ErrInvalidCall
	}
	if err != nil || view.Session.ID != payload.SessionID {
		return roundtable.View{}, controltool.ErrGatewayUnavailable
	}
	if view.Session.Concluded || view.Session.Context == nil || len(view.Rounds) == 0 ||
		view.Rounds[len(view.Rounds)-1].ID != payload.RoundID {
		return roundtable.View{}, controltool.ErrInvalidCall
	}
	if !requireSeat {
		return view, nil
	}
	seat, found := view.Seats[payload.SeatID]
	if !found || !seat.Available || seat.Binding == nil ||
		payload.SeatID == view.Session.ModeratorSeat {
		return roundtable.View{}, controltool.ErrInvalidCall
	}
	return view, nil
}

func newProductConversationControlMetadataGateway(
	sources productConversationControlMetadataSources,
) (*productConversationControlMetadataGateway, error) {
	if nilProductAssetPort(sources.Read) && nilProductAssetPort(sources.Setup) &&
		nilProductAssetPort(sources.Roundtable) && nilProductAssetPort(sources.Incidents) {
		return nil, errProductConversationControlMetadataUnavailable
	}
	return &productConversationControlMetadataGateway{sources: sources}, nil
}

func (gateway *productConversationControlMetadataGateway) Call(
	ctx context.Context,
	turn controltool.TurnContext,
	call controltool.Call,
) (controltool.Result, error) {
	if gateway == nil || ctx == nil || ctx.Err() != nil ||
		!validProductConversationControlMetadataTurn(turn) ||
		!controltool.IsProductMetadataReadTool(call.ToolID) ||
		call.ToolID == controltool.ToolWorkspaceStatus ||
		call.ToolID == controltool.ToolConversationRouteStatus {
		return controltool.Result{}, controltool.ErrInvalidCall
	}
	switch call.ToolID {
	case controltool.ToolMissionsSearch:
		return gateway.missionsSearch(ctx, call.Arguments)
	case controltool.ToolMissionsStatus:
		return gateway.missionStatus(ctx, call.Arguments)
	case controltool.ToolTeamsSearch:
		return gateway.teamsSearch(ctx, call.Arguments)
	case controltool.ToolTeamsStatus:
		return gateway.teamStatus(ctx, call.Arguments)
	case controltool.ToolRoundtablesStatus:
		return gateway.roundtableStatus(ctx, call.Arguments)
	case controltool.ToolGovernanceNeedsYou:
		return gateway.governanceNeedsYou(ctx, call.Arguments)
	case controltool.ToolRuntimesStatus:
		return gateway.runtimesStatus(ctx, call.Arguments)
	case controltool.ToolProvidersStatus:
		return gateway.providersStatus(ctx, call.Arguments)
	case controltool.ToolDiagnosticsIncident:
		return gateway.incidentStatus(ctx, call.Arguments)
	case controltool.ToolLibrarySearch:
		return gateway.librarySearch(ctx, call.Arguments)
	default:
		return controltool.Result{}, controltool.ErrInvalidCall
	}
}

type productConversationControlMissionSummary struct {
	MissionID          string          `json:"mission_id"`
	TeamInstanceID     string          `json:"team_instance_id"`
	Title              string          `json:"title"`
	Lane               api.MissionLane `json:"lane"`
	Status             string          `json:"status"`
	PlanDigest         string          `json:"plan_digest,omitempty"`
	NodeCount          int             `json:"node_count"`
	CompletedNodeCount int             `json:"completed_node_count"`
	ActiveNodeCount    int             `json:"active_node_count"`
	ReviewNodeCount    int             `json:"review_node_count"`
	AttentionCount     int             `json:"attention_count"`
	CurrentNodeID      string          `json:"current_node_id,omitempty"`
	LastMilestone      string          `json:"last_milestone,omitempty"`
	BlockReason        string          `json:"block_reason,omitempty"`
}

func productConversationControlMissionSummaryFor(
	mission api.LocalProductMissionSummary,
) productConversationControlMissionSummary {
	return productConversationControlMissionSummary{
		MissionID: mission.MissionID, TeamInstanceID: mission.TeamInstanceID,
		Title: boundedProductConversationControlText(mission.Title, 256),
		Lane:  mission.Lane, Status: mission.Status, PlanDigest: mission.PlanDigest,
		NodeCount: mission.NodeCount, CompletedNodeCount: mission.CompletedNodeCount,
		ActiveNodeCount: mission.ActiveNodeCount, ReviewNodeCount: mission.ReviewNodeCount,
		AttentionCount: mission.AttentionCount, CurrentNodeID: mission.CurrentNodeID,
		LastMilestone: boundedProductConversationControlText(mission.LastMilestone, 512),
		BlockReason:   boundedProductConversationControlText(mission.BlockReason, 512),
	}
}

func (gateway *productConversationControlMetadataGateway) missionsSearch(
	ctx context.Context,
	arguments json.RawMessage,
) (controltool.Result, error) {
	var input struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if decodeProductConversationControlMetadataArguments(arguments, &input) != nil {
		return controltool.Result{}, controltool.ErrInvalidCall
	}
	query, limit, ok := productConversationControlSearchInput(input.Query, input.Limit)
	if !ok {
		return controltool.Result{}, controltool.ErrInvalidCall
	}
	snapshot, err := gateway.readSnapshot(ctx)
	if err != nil {
		return controltool.Result{}, err
	}
	type match struct {
		value productConversationControlMissionSummary
		exact bool
	}
	matches := make([]match, 0, len(snapshot.Missions))
	for _, mission := range snapshot.Missions {
		matched, exact := productConversationControlMatches(
			query, mission.MissionID, mission.Title,
		)
		if !matched {
			continue
		}
		matches = append(matches, match{
			value: productConversationControlMissionSummaryFor(mission), exact: exact,
		})
	}
	sort.Slice(matches, func(left, right int) bool {
		if matches[left].exact != matches[right].exact {
			return matches[left].exact
		}
		return matches[left].value.MissionID < matches[right].value.MissionID
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	missions := make([]productConversationControlMissionSummary, len(matches))
	for index := range matches {
		missions[index] = matches[index].value
	}
	return productConversationControlMetadataResult(struct {
		SchemaVersion int                                        `json:"schema_version"`
		ViewVersion   string                                     `json:"view_version"`
		Partial       bool                                       `json:"partial"`
		Missions      []productConversationControlMissionSummary `json:"missions"`
	}{1, snapshot.ViewVersion, snapshot.Partial || snapshot.Stale, missions})
}

func (gateway *productConversationControlMetadataGateway) missionStatus(
	ctx context.Context,
	arguments json.RawMessage,
) (controltool.Result, error) {
	var input struct {
		MissionID string `json:"mission_id"`
	}
	if decodeProductConversationControlMetadataArguments(arguments, &input) != nil ||
		!validProductConversationControlOpaque(input.MissionID, 128, false) {
		return controltool.Result{}, controltool.ErrInvalidCall
	}
	snapshot, err := gateway.readSnapshot(ctx)
	if err != nil {
		return controltool.Result{}, err
	}
	var mission *productConversationControlMissionSummary
	for _, candidate := range snapshot.Missions {
		if candidate.MissionID != input.MissionID {
			continue
		}
		value := productConversationControlMissionSummaryFor(candidate)
		mission = &value
		break
	}
	return productConversationControlMetadataResult(struct {
		SchemaVersion int                                       `json:"schema_version"`
		ViewVersion   string                                    `json:"view_version"`
		Found         bool                                      `json:"found"`
		Mission       *productConversationControlMissionSummary `json:"mission,omitempty"`
	}{1, snapshot.ViewVersion, mission != nil, mission})
}

type productConversationControlTeamSummary struct {
	TeamInstanceID        string `json:"team_instance_id"`
	TeamDefinitionID      string `json:"team_definition_id"`
	TeamDefinitionVersion int    `json:"team_definition_version"`
	DisplayName           string `json:"display_name"`
	State                 string `json:"state"`
	Confirmed             bool   `json:"confirmed"`
	Executable            bool   `json:"executable"`
	ReadOnly              bool   `json:"read_only"`
	AgentCount            int    `json:"agent_count"`
}

type productConversationControlTeamAgent struct {
	RoleKind           string `json:"role_kind"`
	AgentDefinitionID  string `json:"agent_definition_id"`
	RuntimeProfileID   string `json:"runtime_profile_id"`
	BindingStatus      string `json:"binding_status"`
	HarnessAdapter     string `json:"harness_adapter"`
	ProviderID         string `json:"provider_id"`
	ProviderAccountID  string `json:"provider_account_id"`
	ModelID            string `json:"model_id"`
	CredentialRevision int64  `json:"credential_revision"`
}

func productConversationControlTeamSummaryFor(
	team api.LocalProductTeamSummary,
) productConversationControlTeamSummary {
	return productConversationControlTeamSummary{
		TeamInstanceID: team.TeamInstanceID, TeamDefinitionID: team.TeamDefinitionID,
		TeamDefinitionVersion: team.TeamDefinitionVersion,
		DisplayName:           boundedProductConversationControlText(team.DisplayName, 256),
		State:                 team.State, Confirmed: team.Confirmed, Executable: team.Executable,
		ReadOnly: team.ReadOnly, AgentCount: len(team.Agents),
	}
}

func productConversationControlTeamAgentsFor(
	team api.LocalProductTeamSummary,
) []productConversationControlTeamAgent {
	agents := make([]productConversationControlTeamAgent, 0, len(team.Agents))
	for _, agent := range team.Agents {
		agents = append(agents, productConversationControlTeamAgent{
			RoleKind: agent.RoleKind, AgentDefinitionID: agent.AgentDefinitionID,
			RuntimeProfileID: agent.RuntimeProfileID, BindingStatus: agent.BindingStatus,
			HarnessAdapter: agent.HarnessAdapter, ProviderID: agent.ProviderID,
			ProviderAccountID: agent.ProviderAccountID, ModelID: agent.ModelID,
			CredentialRevision: agent.CredentialRevision,
		})
	}
	sort.Slice(agents, func(left, right int) bool {
		if agents[left].RoleKind != agents[right].RoleKind {
			return agents[left].RoleKind < agents[right].RoleKind
		}
		return agents[left].AgentDefinitionID < agents[right].AgentDefinitionID
	})
	return agents
}

func (gateway *productConversationControlMetadataGateway) teamsSearch(
	ctx context.Context,
	arguments json.RawMessage,
) (controltool.Result, error) {
	var input struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if decodeProductConversationControlMetadataArguments(arguments, &input) != nil {
		return controltool.Result{}, controltool.ErrInvalidCall
	}
	query, limit, ok := productConversationControlSearchInput(input.Query, input.Limit)
	if !ok {
		return controltool.Result{}, controltool.ErrInvalidCall
	}
	snapshot, err := gateway.readSnapshot(ctx)
	if err != nil {
		return controltool.Result{}, err
	}
	type match struct {
		value productConversationControlTeamSummary
		exact bool
	}
	matches := make([]match, 0, len(snapshot.Teams))
	for _, team := range snapshot.Teams {
		matched, exact := productConversationControlMatches(
			query, team.TeamInstanceID, team.TeamDefinitionID, team.DisplayName,
		)
		if !matched {
			continue
		}
		matches = append(matches, match{
			value: productConversationControlTeamSummaryFor(team), exact: exact,
		})
	}
	sort.Slice(matches, func(left, right int) bool {
		if matches[left].exact != matches[right].exact {
			return matches[left].exact
		}
		return matches[left].value.TeamInstanceID < matches[right].value.TeamInstanceID
	})
	if len(matches) > limit {
		matches = matches[:limit]
	}
	teams := make([]productConversationControlTeamSummary, len(matches))
	for index := range matches {
		teams[index] = matches[index].value
	}
	return productConversationControlMetadataResult(struct {
		SchemaVersion int                                     `json:"schema_version"`
		ViewVersion   string                                  `json:"view_version"`
		Partial       bool                                    `json:"partial"`
		Teams         []productConversationControlTeamSummary `json:"teams"`
	}{1, snapshot.ViewVersion, snapshot.Partial || snapshot.Stale, teams})
}

func (gateway *productConversationControlMetadataGateway) teamStatus(
	ctx context.Context,
	arguments json.RawMessage,
) (controltool.Result, error) {
	var input struct {
		TeamInstanceID string `json:"team_instance_id"`
	}
	if decodeProductConversationControlMetadataArguments(arguments, &input) != nil ||
		!validProductConversationControlOpaque(input.TeamInstanceID, 128, false) {
		return controltool.Result{}, controltool.ErrInvalidCall
	}
	snapshot, err := gateway.readSnapshot(ctx)
	if err != nil {
		return controltool.Result{}, err
	}
	type teamStatus struct {
		productConversationControlTeamSummary
		Agents []productConversationControlTeamAgent `json:"agents"`
	}
	var team *teamStatus
	for _, candidate := range snapshot.Teams {
		if candidate.TeamInstanceID != input.TeamInstanceID {
			continue
		}
		value := teamStatus{
			productConversationControlTeamSummary: productConversationControlTeamSummaryFor(candidate),
			Agents:                                productConversationControlTeamAgentsFor(candidate),
		}
		team = &value
		break
	}
	return productConversationControlMetadataResult(struct {
		SchemaVersion int         `json:"schema_version"`
		ViewVersion   string      `json:"view_version"`
		Found         bool        `json:"found"`
		Team          *teamStatus `json:"team,omitempty"`
	}{1, snapshot.ViewVersion, team != nil, team})
}

type productConversationControlRoundtableRoute struct {
	HarnessAdapter     string `json:"harness_adapter"`
	ProviderID         string `json:"provider_id"`
	ProviderAccountID  string `json:"provider_account_id"`
	CredentialRevision int64  `json:"credential_revision"`
	ModelID            string `json:"model_id"`
	ReasoningEffort    string `json:"reasoning_effort,omitempty"`
	BindingDigest      string `json:"binding_digest"`
}

type productConversationControlRoundtableAttempt struct {
	AttemptID     string `json:"attempt_id"`
	RoundID       string `json:"round_id"`
	Status        string `json:"status"`
	IncidentID    string `json:"incident_id,omitempty"`
	FailureCode   string `json:"failure_code,omitempty"`
	FailureStage  string `json:"failure_stage,omitempty"`
	Retryable     bool   `json:"retryable"`
	AttemptNumber int    `json:"attempt_number"`
}

type productConversationControlRoundtableSeat struct {
	SeatID             string                                       `json:"seat_id"`
	DisplayName        string                                       `json:"display_name"`
	Available          bool                                         `json:"available"`
	AgentDefinitionID  string                                       `json:"agent_definition_id,omitempty"`
	TeamRoleKind       string                                       `json:"team_role_kind,omitempty"`
	RuntimeProfileID   string                                       `json:"runtime_profile_id,omitempty"`
	MembershipRevision int                                          `json:"membership_revision,omitempty"`
	Route              *productConversationControlRoundtableRoute   `json:"route,omitempty"`
	Attempt            *productConversationControlRoundtableAttempt `json:"latest_attempt,omitempty"`
}

func (gateway *productConversationControlMetadataGateway) roundtableStatus(
	ctx context.Context,
	arguments json.RawMessage,
) (controltool.Result, error) {
	var input struct {
		SessionID string `json:"session_id"`
	}
	if decodeProductConversationControlMetadataArguments(arguments, &input) != nil ||
		!validProductConversationControlOpaque(input.SessionID, 128, false) {
		return controltool.Result{}, controltool.ErrInvalidCall
	}
	if nilProductAssetPort(gateway.sources.Roundtable) {
		return controltool.Result{}, controltool.ErrGatewayUnavailable
	}
	view, err := gateway.sources.Roundtable.ReadView(ctx, input.SessionID)
	if errors.Is(err, roundtable.ErrRoundtableSessionNotFound) {
		return productConversationControlMetadataResult(struct {
			SchemaVersion int  `json:"schema_version"`
			Found         bool `json:"found"`
		}{1, false})
	}
	if err != nil || view.Session.ID != input.SessionID {
		return controltool.Result{}, controltool.ErrGatewayUnavailable
	}
	latest := make(map[string]roundtable.SeatAttempt, len(view.Seats))
	for _, attempt := range view.Attempts {
		current, found := latest[attempt.SeatID]
		if !found || attempt.AttemptNumber > current.AttemptNumber ||
			attempt.AttemptNumber == current.AttemptNumber && attempt.StartedAt.After(current.StartedAt) {
			latest[attempt.SeatID] = attempt
		}
	}
	seatIDs := make([]string, 0, len(view.Seats))
	for seatID := range view.Seats {
		seatIDs = append(seatIDs, seatID)
	}
	sort.Strings(seatIDs)
	seats := make([]productConversationControlRoundtableSeat, 0, len(seatIDs))
	for _, seatID := range seatIDs {
		seat := view.Seats[seatID]
		item := productConversationControlRoundtableSeat{
			SeatID: seat.ID, DisplayName: boundedProductConversationControlText(seat.DisplayName, 128),
			Available: seat.Available,
		}
		if seat.Binding != nil {
			item.AgentDefinitionID = seat.Binding.AgentDefinitionID
			item.TeamRoleKind = seat.Binding.TeamRoleKind
			item.RuntimeProfileID = seat.Binding.RuntimeProfileID
			item.MembershipRevision = seat.Binding.MembershipRevision
			binding := seat.Binding.ExecutionBinding
			item.Route = &productConversationControlRoundtableRoute{
				HarnessAdapter: binding.HarnessAdapter, ProviderID: binding.ProviderID,
				ProviderAccountID:  binding.ProviderAccountID,
				CredentialRevision: binding.CredentialRevision, ModelID: binding.ModelID,
				ReasoningEffort: binding.ReasoningEffort, BindingDigest: binding.BindingDigest,
			}
		}
		if attempt, found := latest[seatID]; found {
			item.Attempt = &productConversationControlRoundtableAttempt{
				AttemptID: attempt.AttemptID, RoundID: attempt.RoundID, Status: attempt.Status,
				IncidentID: attempt.IncidentID, FailureCode: attempt.FailureCode,
				FailureStage: attempt.FailureStage, Retryable: attempt.Retryable,
				AttemptNumber: attempt.AttemptNumber,
			}
		}
		seats = append(seats, item)
	}
	rounds := append([]roundtable.Round(nil), view.Rounds...)
	sort.Slice(rounds, func(left, right int) bool { return rounds[left].Sequence < rounds[right].Sequence })
	if len(rounds) > productConversationControlMetadataMaximumLimit {
		rounds = rounds[len(rounds)-productConversationControlMetadataMaximumLimit:]
	}
	type roundSummary struct {
		RoundID        string `json:"round_id"`
		Sequence       int    `json:"sequence"`
		MessageCount   int    `json:"message_count"`
		PauseRequested bool   `json:"pause_requested"`
	}
	roundSummaries := make([]roundSummary, len(rounds))
	for index, current := range rounds {
		roundSummaries[index] = roundSummary{
			RoundID: current.ID, Sequence: current.Sequence,
			MessageCount: current.MessageCount, PauseRequested: current.PauseRequested,
		}
	}
	pendingMessages := 0
	for _, message := range view.Messages {
		if message.Status == roundtable.MessagePending || message.Status == roundtable.MessageRelayed {
			pendingMessages++
		}
	}
	type sessionContext struct {
		ConversationID string `json:"conversation_id"`
		MissionID      string `json:"mission_id"`
		TeamID         string `json:"team_id"`
		TeamVersion    int    `json:"team_version"`
		WorkspaceID    string `json:"workspace_id"`
	}
	var linked *sessionContext
	if view.Session.Context != nil {
		linked = &sessionContext{
			ConversationID: view.Session.Context.ConversationID,
			MissionID:      view.Session.Context.MissionID, TeamID: view.Session.Context.TeamID,
			TeamVersion: view.Session.Context.TeamVersion,
			WorkspaceID: view.Session.Context.WorkspaceID,
		}
	}
	return productConversationControlMetadataResult(struct {
		SchemaVersion     int                                        `json:"schema_version"`
		Found             bool                                       `json:"found"`
		SessionID         string                                     `json:"session_id"`
		Title             string                                     `json:"title"`
		ModeratorSeat     string                                     `json:"moderator_seat"`
		Concluded         bool                                       `json:"concluded"`
		Context           *sessionContext                            `json:"context,omitempty"`
		Seats             []productConversationControlRoundtableSeat `json:"seats"`
		Rounds            []roundSummary                             `json:"rounds"`
		PendingMessages   int                                        `json:"pending_message_count"`
		InterventionCount int                                        `json:"intervention_count"`
		ViewDigest        string                                     `json:"view_digest"`
	}{
		1, true, view.Session.ID,
		boundedProductConversationControlText(view.Session.Title, 256),
		view.Session.ModeratorSeat, view.Session.Concluded, linked, seats,
		roundSummaries, pendingMessages, len(view.Interventions), view.Digest,
	})
}

func (gateway *productConversationControlMetadataGateway) governanceNeedsYou(
	ctx context.Context,
	arguments json.RawMessage,
) (controltool.Result, error) {
	var input struct {
		Limit int `json:"limit"`
	}
	if decodeProductConversationControlMetadataArguments(arguments, &input) != nil {
		return controltool.Result{}, controltool.ErrInvalidCall
	}
	limit, ok := productConversationControlLimit(input.Limit)
	if !ok {
		return controltool.Result{}, controltool.ErrInvalidCall
	}
	snapshot, err := gateway.readSnapshot(ctx)
	if err != nil {
		return controltool.Result{}, err
	}
	attention := append([]api.AttentionItem(nil), snapshot.Attention...)
	sort.Slice(attention, func(left, right int) bool {
		return attention[left].AttentionID < attention[right].AttentionID
	})
	if len(attention) > limit {
		attention = attention[:limit]
	}
	type attentionSummary struct {
		AttentionID    string `json:"attention_id"`
		Kind           string `json:"kind"`
		Severity       string `json:"severity"`
		TeamInstanceID string `json:"team_instance_id,omitempty"`
		LogicalNodeID  string `json:"logical_node_id,omitempty"`
		Status         string `json:"status"`
		ActionRequired string `json:"action_required"`
	}
	items := make([]attentionSummary, len(attention))
	for index, item := range attention {
		items[index] = attentionSummary{
			AttentionID: item.AttentionID, Kind: item.Kind, Severity: item.Severity,
			TeamInstanceID: item.TeamInstanceID, LogicalNodeID: item.LogicalNodeID,
			Status:         item.Status,
			ActionRequired: boundedProductConversationControlText(item.ActionRequired, 512),
		}
	}
	blocked := make([]productConversationControlMissionSummary, 0, limit)
	for _, mission := range snapshot.Missions {
		if mission.BlockReason == "" && mission.AttentionCount == 0 &&
			mission.Status != "blocked" && mission.Status != "failed" {
			continue
		}
		blocked = append(blocked, productConversationControlMissionSummaryFor(mission))
	}
	sort.Slice(blocked, func(left, right int) bool {
		return blocked[left].MissionID < blocked[right].MissionID
	})
	if len(blocked) > limit {
		blocked = blocked[:limit]
	}
	return productConversationControlMetadataResult(struct {
		SchemaVersion int                                        `json:"schema_version"`
		ViewVersion   string                                     `json:"view_version"`
		Attention     []attentionSummary                         `json:"attention"`
		Blocked       []productConversationControlMissionSummary `json:"blocked_missions"`
	}{1, snapshot.ViewVersion, items, blocked})
}

func (gateway *productConversationControlMetadataGateway) runtimesStatus(
	ctx context.Context,
	arguments json.RawMessage,
) (controltool.Result, error) {
	var input struct {
		AdapterType string `json:"adapter_type"`
		Limit       int    `json:"limit"`
	}
	if decodeProductConversationControlMetadataArguments(arguments, &input) != nil ||
		!validProductConversationControlOpaque(input.AdapterType, 64, true) {
		return controltool.Result{}, controltool.ErrInvalidCall
	}
	limit, ok := productConversationControlLimit(input.Limit)
	if !ok {
		return controltool.Result{}, controltool.ErrInvalidCall
	}
	snapshot, err := gateway.readSnapshot(ctx)
	if err != nil {
		return controltool.Result{}, err
	}
	runtimes := make([]api.LocalProductRuntimeSummary, 0, len(snapshot.Runtimes))
	for _, runtime := range snapshot.Runtimes {
		if input.AdapterType != "" && runtime.AdapterType != input.AdapterType {
			continue
		}
		runtime.ModelIDs = productConversationControlBoundedStrings(runtime.ModelIDs, 32, 256)
		runtime.ObservedCapabilities = productConversationControlBoundedStrings(
			runtime.ObservedCapabilities, 32, 128,
		)
		runtimes = append(runtimes, runtime)
	}
	sort.Slice(runtimes, func(left, right int) bool {
		return runtimes[left].RuntimeInstanceID < runtimes[right].RuntimeInstanceID
	})
	if len(runtimes) > limit {
		runtimes = runtimes[:limit]
	}
	return productConversationControlMetadataResult(struct {
		SchemaVersion int                              `json:"schema_version"`
		ViewVersion   string                           `json:"view_version"`
		Partial       bool                             `json:"partial"`
		Runtimes      []api.LocalProductRuntimeSummary `json:"runtimes"`
	}{1, snapshot.ViewVersion, snapshot.Partial || snapshot.Stale, runtimes})
}

type productConversationControlProvider struct {
	ProviderID             string `json:"provider_id"`
	DisplayName            string `json:"display_name"`
	Category               string `json:"category,omitempty"`
	Protocol               string `json:"protocol"`
	AuthMode               string `json:"auth_mode"`
	ConnectionKind         string `json:"connection_kind"`
	Revision               int64  `json:"credential_revision"`
	Status                 string `json:"status"`
	Reason                 string `json:"reason,omitempty"`
	SupportsModelDiscovery bool   `json:"supports_model_discovery"`
}

type productConversationControlProviderAccount struct {
	ProviderID                 string `json:"provider_id"`
	ProviderAccountID          string `json:"provider_account_id"`
	Revision                   int64  `json:"credential_revision"`
	Status                     string `json:"status"`
	Reason                     string `json:"reason,omitempty"`
	PolicyAvailable            bool   `json:"policy_available"`
	PolicyVersion              int    `json:"policy_version,omitempty"`
	PolicyRevision             int64  `json:"policy_revision,omitempty"`
	PolicyDigest               string `json:"policy_digest,omitempty"`
	MaximumConcurrentAttempts  int    `json:"maximum_concurrent_attempts,omitempty"`
	MaximumAssignedBudgetUnits int64  `json:"maximum_assigned_budget_units,omitempty"`
	TrustDomain                string `json:"trust_domain,omitempty"`
	RetentionMode              string `json:"retention_mode,omitempty"`
	DataRegion                 string `json:"data_region,omitempty"`
}

type productConversationControlProfile struct {
	ProfileID          string `json:"profile_id"`
	HarnessAdapter     string `json:"harness_adapter"`
	ProviderID         string `json:"provider_id"`
	ProviderAccountID  string `json:"provider_account_id"`
	DisplayName        string `json:"display_name"`
	ModelID            string `json:"model_id"`
	CredentialRevision int64  `json:"credential_revision"`
	PolicyVersion      int    `json:"policy_version,omitempty"`
	PolicyRevision     int64  `json:"policy_revision,omitempty"`
	PolicyDigest       string `json:"policy_digest,omitempty"`
	TrustDomain        string `json:"trust_domain,omitempty"`
	RetentionMode      string `json:"retention_mode,omitempty"`
	DataRegion         string `json:"data_region,omitempty"`
}

func (gateway *productConversationControlMetadataGateway) providersStatus(
	ctx context.Context,
	arguments json.RawMessage,
) (controltool.Result, error) {
	var input struct {
		ProviderID        string `json:"provider_id"`
		ProviderAccountID string `json:"provider_account_id"`
		Limit             int    `json:"limit"`
	}
	if decodeProductConversationControlMetadataArguments(arguments, &input) != nil ||
		!validProductConversationControlOpaque(input.ProviderID, 64, true) ||
		!validProductConversationControlOpaque(input.ProviderAccountID, 128, true) {
		return controltool.Result{}, controltool.ErrInvalidCall
	}
	limit, ok := productConversationControlLimit(input.Limit)
	if !ok {
		return controltool.Result{}, controltool.ErrInvalidCall
	}
	if nilProductAssetPort(gateway.sources.Setup) {
		return controltool.Result{}, controltool.ErrGatewayUnavailable
	}
	snapshot, err := gateway.sources.Setup.SetupSnapshot(ctx)
	if err != nil || snapshot.SchemaVersion <= 0 {
		return controltool.Result{}, controltool.ErrGatewayUnavailable
	}
	providers := make([]productConversationControlProvider, 0, len(snapshot.Providers))
	for _, provider := range snapshot.Providers {
		if input.ProviderID != "" && provider.ProviderID != input.ProviderID {
			continue
		}
		providers = append(providers, productConversationControlProvider{
			ProviderID:  provider.ProviderID,
			DisplayName: boundedProductConversationControlText(provider.DisplayName, 128),
			Category:    provider.Category, Protocol: provider.Protocol, AuthMode: provider.AuthMode,
			ConnectionKind: provider.ConnectionKind, Revision: provider.Revision,
			Status:                 provider.Status,
			Reason:                 boundedProductConversationControlText(provider.Reason, 256),
			SupportsModelDiscovery: provider.SupportsModelDiscovery,
		})
	}
	accounts := make([]productConversationControlProviderAccount, 0, len(snapshot.ProviderAccounts))
	for _, account := range snapshot.ProviderAccounts {
		if input.ProviderID != "" && account.ProviderID != input.ProviderID ||
			input.ProviderAccountID != "" && account.ProviderAccountID != input.ProviderAccountID {
			continue
		}
		accounts = append(accounts, productConversationControlProviderAccount{
			ProviderID: account.ProviderID, ProviderAccountID: account.ProviderAccountID,
			Revision: account.Revision, Status: account.Status,
			Reason:          boundedProductConversationControlText(account.Reason, 256),
			PolicyAvailable: account.PolicyAvailable, PolicyVersion: account.PolicyVersion,
			PolicyRevision: account.PolicyRevision, PolicyDigest: account.PolicyDigest,
			MaximumConcurrentAttempts:  account.MaximumConcurrentAttempts,
			MaximumAssignedBudgetUnits: account.MaximumAssignedBudgetUnits,
			TrustDomain:                account.TrustDomain, RetentionMode: account.RetentionMode,
			DataRegion: account.DataRegion,
		})
	}
	profiles := make([]productConversationControlProfile, 0, len(snapshot.ConversationProfiles))
	for _, profile := range snapshot.ConversationProfiles {
		if input.ProviderID != "" && profile.ProviderID != input.ProviderID ||
			input.ProviderAccountID != "" && profile.ProviderAccountID != input.ProviderAccountID {
			continue
		}
		profiles = append(profiles, productConversationControlProfile{
			ProfileID: profile.ProfileID, HarnessAdapter: profile.HarnessAdapter,
			ProviderID: profile.ProviderID, ProviderAccountID: profile.ProviderAccountID,
			DisplayName: boundedProductConversationControlText(profile.DisplayName, 128),
			ModelID:     profile.ModelID, CredentialRevision: profile.CredentialRevision,
			PolicyVersion: profile.PolicyVersion, PolicyRevision: profile.PolicyRevision,
			PolicyDigest: profile.PolicyDigest, TrustDomain: profile.TrustDomain,
			RetentionMode: profile.RetentionMode, DataRegion: profile.DataRegion,
		})
	}
	sort.Slice(providers, func(left, right int) bool {
		return providers[left].ProviderID < providers[right].ProviderID
	})
	sort.Slice(accounts, func(left, right int) bool {
		if accounts[left].ProviderID != accounts[right].ProviderID {
			return accounts[left].ProviderID < accounts[right].ProviderID
		}
		return accounts[left].ProviderAccountID < accounts[right].ProviderAccountID
	})
	sort.Slice(profiles, func(left, right int) bool {
		return profiles[left].ProfileID < profiles[right].ProfileID
	})
	if len(providers) > limit {
		providers = providers[:limit]
	}
	if len(accounts) > limit {
		accounts = accounts[:limit]
	}
	if len(profiles) > limit {
		profiles = profiles[:limit]
	}
	return productConversationControlMetadataResult(struct {
		SchemaVersion int                                         `json:"schema_version"`
		ViewVersion   string                                      `json:"view_version"`
		Providers     []productConversationControlProvider        `json:"providers"`
		Accounts      []productConversationControlProviderAccount `json:"provider_accounts"`
		Profiles      []productConversationControlProfile         `json:"conversation_profiles"`
	}{1, snapshot.ViewVersion, providers, accounts, profiles})
}

func (gateway *productConversationControlMetadataGateway) incidentStatus(
	ctx context.Context,
	arguments json.RawMessage,
) (controltool.Result, error) {
	var input struct {
		IncidentID string `json:"incident_id"`
	}
	if decodeProductConversationControlMetadataArguments(arguments, &input) != nil ||
		!validProductConversationControlOpaque(input.IncidentID, 128, false) {
		return controltool.Result{}, controltool.ErrInvalidCall
	}
	if nilProductAssetPort(gateway.sources.Incidents) {
		return controltool.Result{}, controltool.ErrGatewayUnavailable
	}
	events, err := gateway.sources.Incidents.IncidentDiagnostics(ctx, input.IncidentID)
	if err != nil {
		return controltool.Result{}, controltool.ErrGatewayUnavailable
	}
	filtered := make([]productIncidentDiagnosticEvent, 0, len(events))
	for _, event := range events {
		if event.IncidentID != input.IncidentID || event.Stage == "" || event.Result == "" {
			continue
		}
		filtered = append(filtered, event)
	}
	sort.Slice(filtered, func(left, right int) bool {
		if filtered[left].OccurredAt != filtered[right].OccurredAt {
			return filtered[left].OccurredAt < filtered[right].OccurredAt
		}
		return filtered[left].Operation < filtered[right].Operation
	})
	if len(filtered) > productConversationControlIncidentMaximumEvents {
		filtered = filtered[len(filtered)-productConversationControlIncidentMaximumEvents:]
	}
	return productConversationControlMetadataResult(struct {
		SchemaVersion int                              `json:"schema_version"`
		IncidentID    string                           `json:"incident_id"`
		Found         bool                             `json:"found"`
		Events        []productIncidentDiagnosticEvent `json:"events"`
	}{1, input.IncidentID, len(filtered) > 0, filtered})
}

func (gateway *productConversationControlMetadataGateway) librarySearch(
	ctx context.Context,
	arguments json.RawMessage,
) (controltool.Result, error) {
	var input struct {
		Query string `json:"query"`
		Limit int    `json:"limit"`
	}
	if decodeProductConversationControlMetadataArguments(arguments, &input) != nil {
		return controltool.Result{}, controltool.ErrInvalidCall
	}
	query, limit, ok := productConversationControlSearchInput(input.Query, input.Limit)
	if !ok {
		return controltool.Result{}, controltool.ErrInvalidCall
	}
	snapshot, err := gateway.readSnapshot(ctx)
	if err != nil {
		return controltool.Result{}, err
	}
	evidence := make([]api.LocalProductEvidenceSummary, 0, len(snapshot.Evidence))
	for _, item := range snapshot.Evidence {
		matched, _ := productConversationControlMatches(
			query, item.EvidenceID, item.WorkItemID, item.Digest,
		)
		if matched {
			evidence = append(evidence, item)
		}
	}
	sort.Slice(evidence, func(left, right int) bool {
		return evidence[left].EvidenceID < evidence[right].EvidenceID
	})
	if len(evidence) > limit {
		evidence = evidence[:limit]
	}
	return productConversationControlMetadataResult(struct {
		SchemaVersion int                               `json:"schema_version"`
		ViewVersion   string                            `json:"view_version"`
		Evidence      []api.LocalProductEvidenceSummary `json:"evidence"`
	}{1, snapshot.ViewVersion, evidence})
}

func (gateway *productConversationControlMetadataGateway) readSnapshot(
	ctx context.Context,
) (api.LocalProductSnapshot, error) {
	if gateway == nil || nilProductAssetPort(gateway.sources.Read) {
		return api.LocalProductSnapshot{}, controltool.ErrGatewayUnavailable
	}
	snapshot, err := gateway.sources.Read.ReadLocalProductSnapshot(
		ctx,
		api.LocalProductSnapshotRequest{
			Limit: productConversationControlMetadataSnapshotLimit,
		},
	)
	if err != nil || snapshot.SchemaVersion <= 0 {
		return api.LocalProductSnapshot{}, controltool.ErrGatewayUnavailable
	}
	return snapshot, nil
}

func productConversationControlMetadataResult(
	value any,
) (controltool.Result, error) {
	body, err := json.Marshal(value)
	if err != nil || len(body) == 0 ||
		len(body) > productConversationControlMetadataMaximumResult {
		return controltool.Result{}, controltool.ErrGatewayUnavailable
	}
	return controltool.Result{Content: body}, nil
}

func productConversationControlSearchInput(
	query string,
	requestedLimit int,
) (string, int, bool) {
	if !utf8.ValidString(query) || len(query) > 256 ||
		strings.IndexByte(query, 0) >= 0 || productConversationControlHasControl(query) {
		return "", 0, false
	}
	limit, ok := productConversationControlLimit(requestedLimit)
	if !ok {
		return "", 0, false
	}
	return strings.ToLower(strings.TrimSpace(query)), limit, true
}

func productConversationControlLimit(requested int) (int, bool) {
	if requested < 0 || requested > productConversationControlMetadataMaximumLimit {
		return 0, false
	}
	if requested == 0 {
		return productConversationControlMetadataDefaultLimit, true
	}
	return requested, true
}

func productConversationControlMatches(
	query string,
	values ...string,
) (bool, bool) {
	if query == "" {
		return true, false
	}
	matched := false
	for _, value := range values {
		normalized := strings.ToLower(value)
		if normalized == query {
			return true, true
		}
		matched = matched || strings.Contains(normalized, query)
	}
	return matched, false
}

func productConversationControlBoundedStrings(
	values []string,
	maximumItems int,
	maximumBytes int,
) []string {
	if len(values) > maximumItems {
		values = values[:maximumItems]
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if bounded := boundedProductConversationControlText(value, maximumBytes); bounded != "" {
			result = append(result, bounded)
		}
	}
	return result
}

func boundedProductConversationControlText(value string, maximum int) string {
	if value == "" || !utf8.ValidString(value) || strings.IndexByte(value, 0) >= 0 {
		return ""
	}
	value = strings.TrimSpace(value)
	if value == "" || productConversationControlHasControl(value) {
		return ""
	}
	if len(value) <= maximum {
		return value
	}
	value = value[:maximum]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return strings.TrimSpace(value)
}

func validProductConversationControlMetadataTurn(
	turn controltool.TurnContext,
) bool {
	registry, err := controltool.NewBuiltinRegistry()
	if err != nil {
		return false
	}
	return validProductConversationControlOpaque(turn.ConversationID, 256, false) &&
		validProductConversationControlOpaque(turn.SegmentID, 128, false) &&
		validProductConversationControlOpaque(turn.AttemptID, 128, false) &&
		validProductConversationControlOpaque(turn.IncidentID, 128, false) &&
		turn.Route.Valid() && turn.Workspace.Valid() &&
		turn.RegistryDigest == registry.Digest()
}

func validProductConversationControlOpaque(
	value string,
	maximum int,
	allowEmpty bool,
) bool {
	if value == "" {
		return allowEmpty
	}
	return len(value) <= maximum && utf8.ValidString(value) &&
		strings.TrimSpace(value) == value && strings.IndexByte(value, 0) < 0 &&
		!productConversationControlHasControl(value)
}

func productConversationControlHasControl(value string) bool {
	for _, current := range value {
		if unicode.IsControl(current) {
			return true
		}
	}
	return false
}

func decodeProductConversationControlMetadataArguments(
	body []byte,
	value any,
) error {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) < 2 || len(trimmed) > 32<<10 || !utf8.Valid(trimmed) ||
		trimmed[0] != '{' || trimmed[len(trimmed)-1] != '}' ||
		rejectProductConversationControlDuplicateJSONKeys(trimmed) {
		return controltool.ErrInvalidCall
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	if decoder.Decode(value) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return controltool.ErrInvalidCall
	}
	return nil
}

func rejectProductConversationControlDuplicateJSONKeys(payload []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	var walk func() bool
	walk = func() bool {
		token, err := decoder.Token()
		if err != nil {
			return true
		}
		delimiter, composite := token.(json.Delim)
		if !composite {
			return false
		}
		switch delimiter {
		case '{':
			seen := map[string]struct{}{}
			for decoder.More() {
				keyToken, keyErr := decoder.Token()
				key, ok := keyToken.(string)
				if keyErr != nil || !ok {
					return true
				}
				if _, duplicate := seen[key]; duplicate {
					return true
				}
				seen[key] = struct{}{}
				if walk() {
					return true
				}
			}
		case '[':
			for decoder.More() {
				if walk() {
					return true
				}
			}
		default:
			return true
		}
		_, err = decoder.Token()
		return err != nil
	}
	if walk() {
		return true
	}
	return decoder.Decode(&struct{}{}) != io.EOF
}

var _ controltool.Gateway = (*productConversationControlMetadataGateway)(nil)
