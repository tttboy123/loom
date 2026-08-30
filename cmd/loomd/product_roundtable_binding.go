package main

import (
	"context"
	"strings"

	"loom-pi-rebuild/internal/agents"
	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/roundtable"
	"loom-pi-rebuild/internal/teams"
)

type productRoundtableMissionContextSource interface {
	ResolveRoundtableMissionContext(
		context.Context,
		string,
	) (app.RoundtableMissionContext, error)
}

// productProjectedRoundtableBindingResolver is the sole product admission
// path that turns client-selected Team identities into immutable RoundTable
// execution bindings. It never accepts Provider or credential fields from IPC.
type productProjectedRoundtableBindingResolver struct {
	projection *projection.Projection
}

func newProductProjectedRoundtableBindingResolver(
	readModel *projection.Projection,
) productRoundtableBindingResolver {
	if readModel == nil {
		return nil
	}
	return &productProjectedRoundtableBindingResolver{projection: readModel}
}

func newProductProjectedRoundtableMissionContextSource(
	readModel *projection.Projection,
) productRoundtableMissionContextSource {
	if readModel == nil {
		return nil
	}
	return &productProjectedRoundtableBindingResolver{projection: readModel}
}

func (resolver *productProjectedRoundtableBindingResolver) ResolveRoundtableMissionContext(
	ctx context.Context,
	missionID string,
) (app.RoundtableMissionContext, error) {
	if resolver == nil || resolver.projection == nil || ctx == nil ||
		!strings.HasPrefix(missionID, "mission/") {
		return app.RoundtableMissionContext{}, roundtable.ErrInvalidRoundtableSession
	}
	if err := resolver.projection.Rebuild(ctx); err != nil {
		return app.RoundtableMissionContext{}, err
	}
	teamID := strings.TrimPrefix(missionID, "mission/")
	execution, found := resolver.projection.GlobalReadView().TeamExecution(teamID)
	if !found {
		return app.RoundtableMissionContext{}, roundtable.ErrInvalidRoundtableSession
	}
	return productRoundtableMissionContext(missionID, execution)
}

func productRoundtableMissionContext(
	missionID string,
	execution projection.TeamExecution,
) (app.RoundtableMissionContext, error) {
	if missionID == "" || execution.TeamInstanceID == "" ||
		missionID != "mission/"+execution.TeamInstanceID ||
		execution.PlanDigest == "" || execution.Status == "" {
		return app.RoundtableMissionContext{}, roundtable.ErrInvalidRoundtableSession
	}
	objective := ""
	for _, node := range execution.Nodes {
		if node.Role != string(teams.ExecutionRoleMain) ||
			node.Kind == string(teams.ExecutionNodeRouteSibling) {
			continue
		}
		if objective != "" || strings.TrimSpace(node.Title) == "" {
			return app.RoundtableMissionContext{}, roundtable.ErrInvalidRoundtableSession
		}
		objective = node.Title
	}
	if objective == "" {
		return app.RoundtableMissionContext{}, roundtable.ErrInvalidRoundtableSession
	}
	return app.RoundtableMissionContext{
		MissionID: missionID, TeamInstanceID: execution.TeamInstanceID,
		Objective: objective, Status: execution.Status, PlanDigest: execution.PlanDigest,
	}, nil
}

func (resolver *productProjectedRoundtableBindingResolver) ResolveSessionContext(
	ctx context.Context,
	request roundtable.SessionLinkRequest,
) (roundtable.SessionContext, error) {
	if resolver == nil || resolver.projection == nil || ctx == nil ||
		request.ConversationID == "" || request.MissionID == "" ||
		request.TeamInstanceID == "" {
		return roundtable.SessionContext{}, roundtable.ErrInvalidRoundtableSession
	}
	if err := resolver.projection.Rebuild(ctx); err != nil {
		return roundtable.SessionContext{}, err
	}
	view := resolver.projection.GlobalReadView()
	team, found := view.Team(request.TeamInstanceID)
	if !found || team.ID != request.TeamInstanceID ||
		request.MissionID != "mission/"+team.ID ||
		team.TeamDefinitionID == "" || team.TeamDefinitionVersion <= 0 {
		return roundtable.SessionContext{}, roundtable.ErrInvalidRoundtableSession
	}
	definition, found := view.TeamDefinition(team.TeamDefinitionID)
	if !found || definition.ID != team.TeamDefinitionID ||
		definition.Version != team.TeamDefinitionVersion ||
		definition.DefinitionDigest != team.TeamDefinitionDigest {
		return roundtable.SessionContext{}, roundtable.ErrInvalidRoundtableSession
	}
	workspaceID := team.ScopeIdentity.ProjectID
	if workspaceID == "" {
		workspaceID = "local"
	}
	return roundtable.SessionContext{
		ConversationID: request.ConversationID,
		MissionID:      request.MissionID,
		TeamID:         team.ID,
		TeamVersion:    team.TeamDefinitionVersion,
		WorkspaceID:    workspaceID,
	}, nil
}

func (resolver *productProjectedRoundtableBindingResolver) ResolveSeatBinding(
	ctx context.Context,
	sessionID string,
	seatID string,
	sessionContext roundtable.SessionContext,
	request roundtable.SeatBindingRequest,
	membershipRevision int,
) (roundtable.FrozenSeatBinding, error) {
	if resolver == nil || resolver.projection == nil || ctx == nil ||
		request.AgentDefinitionID == "" || request.RuntimeProfileID == "" ||
		(request.TeamRoleKind != "main" && request.TeamRoleKind != "subagent") ||
		membershipRevision <= 0 {
		return roundtable.FrozenSeatBinding{}, roundtable.ErrInvalidRoundtableSeatBinding
	}
	if err := resolver.projection.Rebuild(ctx); err != nil {
		return roundtable.FrozenSeatBinding{}, err
	}
	view := resolver.projection.GlobalReadView()
	team, found := view.Team(sessionContext.TeamID)
	if !found || team.ID != sessionContext.TeamID ||
		sessionContext.MissionID != "mission/"+team.ID ||
		team.TeamDefinitionVersion != sessionContext.TeamVersion {
		return roundtable.FrozenSeatBinding{}, roundtable.ErrInvalidRoundtableSeatBinding
	}
	record, found := view.TeamDefinition(team.TeamDefinitionID)
	if !found || record.ID != team.TeamDefinitionID ||
		record.Version != team.TeamDefinitionVersion ||
		record.DefinitionDigest != team.TeamDefinitionDigest {
		return roundtable.FrozenSeatBinding{}, roundtable.ErrInvalidRoundtableSeatBinding
	}
	catalog, err := productSetupCatalogForView(ctx, view)
	if err != nil {
		return roundtable.FrozenSeatBinding{}, err
	}
	definition, selections, profiles, err := productSavedTeamMaterializationInputs(
		record, catalog,
	)
	if err != nil {
		return roundtable.FrozenSeatBinding{}, roundtable.ErrInvalidRoundtableSeatBinding
	}
	binding, err := teams.BuildSavedTeamRuntimeBinding(
		[]teams.TeamDefinition{definition}, definition.ID(),
		agents.ScopeIdentity{
			ProjectID:    team.ScopeIdentity.ProjectID,
			GenerationID: team.ScopeIdentity.GenerationID,
		},
		catalog.AgentDefinitions, profiles, catalog.RuntimeDiscovery, selections,
	)
	if err != nil {
		return roundtable.FrozenSeatBinding{}, roundtable.ErrInvalidRoundtableSeatBinding
	}
	role, found := productRoundtableRoleBinding(binding, request)
	if !found {
		return roundtable.FrozenSeatBinding{}, roundtable.ErrInvalidRoundtableSeatBinding
	}
	frozen, err := roundtable.FreezeSeatBinding(
		sessionID, seatID, sessionContext, role.AgentDefinitionID,
		string(role.Kind), role.RuntimeProfileID, role.ExecutionBinding,
		membershipRevision,
	)
	if err != nil {
		return roundtable.FrozenSeatBinding{}, err
	}
	return frozen, nil
}

func productRoundtableRoleBinding(
	binding teams.SavedTeamRuntimeBindingCandidate,
	request roundtable.SeatBindingRequest,
) (teams.SavedTeamRuntimeRoleBinding, bool) {
	candidates := append(
		[]teams.SavedTeamRuntimeRoleBinding{binding.MainBinding()},
		binding.SubAgentBindings()...,
	)
	for _, candidate := range candidates {
		if candidate.AgentDefinitionID == request.AgentDefinitionID &&
			string(candidate.Kind) == request.TeamRoleKind &&
			candidate.RuntimeProfileID == request.RuntimeProfileID {
			return candidate, true
		}
	}
	return teams.SavedTeamRuntimeRoleBinding{}, false
}

var _ productRoundtableBindingResolver = (*productProjectedRoundtableBindingResolver)(nil)
var _ productRoundtableMissionContextSource = (*productProjectedRoundtableBindingResolver)(nil)
