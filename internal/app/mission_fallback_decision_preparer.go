package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"loom-pi-rebuild/internal/projection"
	"loom-pi-rebuild/internal/state"
	"loom-pi-rebuild/internal/work"
)

type MissionFallbackDecisionCandidate struct {
	MissionID          string
	Scope              work.TeamFallbackDecisionScope
	AgentTitle         string
	HarnessAdapter     string
	ProviderID         string
	ProviderAccountID  string
	ModelID            string
	CredentialRevision int64
}

type MissionFallbackDecisionPreparer interface {
	PrepareMissionFallbackDecisions(
		context.Context,
		string,
		string,
		[]MissionFallbackDecisionCandidate,
	) error
}

type ProjectionMissionFallbackDecisionPreparerConfig struct {
	Backend    *PreparedMissionDecisionBackend
	Projection *projection.Projection
	Authority  MissionFallbackDecisionAuthority
	ActorRef   string
	Now        func() time.Time
}

type ProjectionMissionFallbackDecisionPreparer struct {
	backend    *PreparedMissionDecisionBackend
	projection *projection.Projection
	authority  MissionFallbackDecisionAuthority
	actorRef   string
	now        func() time.Time
}

func NewProjectionMissionFallbackDecisionPreparer(
	config ProjectionMissionFallbackDecisionPreparerConfig,
) (*ProjectionMissionFallbackDecisionPreparer, error) {
	if config.Backend == nil || config.Projection == nil ||
		config.Authority == nil || !validDecisionID(config.ActorRef) ||
		config.Now == nil {
		return nil, ErrInvalidMissionDecision
	}
	now := config.Now()
	if now.IsZero() || now.Location() != time.UTC {
		return nil, ErrInvalidMissionDecision
	}
	return &ProjectionMissionFallbackDecisionPreparer{
		backend: config.Backend, projection: config.Projection,
		authority: config.Authority, actorRef: config.ActorRef, now: config.Now,
	}, nil
}

func (preparer *ProjectionMissionFallbackDecisionPreparer) PrepareMissionFallbackDecisions(
	ctx context.Context,
	teamInstanceID string,
	viewVersion string,
	candidates []MissionFallbackDecisionCandidate,
) error {
	if preparer == nil || preparer.backend == nil ||
		preparer.projection == nil || preparer.authority == nil ||
		ctx == nil || !validDecisionID(teamInstanceID) ||
		!validDecisionDigest(viewVersion) {
		return ErrInvalidMissionDecision
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if preparer.projection.GlobalReadView().Version() != viewVersion {
		return ErrMissionDecisionConflict
	}
	prepared := make([]PreparedMissionFallbackDecision, 0, len(candidates))
	for _, candidate := range candidates {
		if !validMissionFallbackDecisionCandidate(candidate) ||
			candidate.Scope.TeamInstanceID() != teamInstanceID {
			return ErrInvalidMissionDecision
		}
		expectedRevision := int64(0)
		if current, ok := preparer.projection.MissionFallbackDecision(
			candidate.Scope,
		); ok {
			if current.Revision <= 0 ||
				(current.Decision != "approved" && current.Decision != "rejected") {
				return ErrMissionDecisionConflict
			}
			expectedRevision = current.Revision
		}
		decision, err := preparer.prepare(
			candidate, viewVersion, expectedRevision,
		)
		if err != nil {
			return err
		}
		prepared = append(prepared, decision)
	}
	return preparer.backend.replacePreparedMissionFallbackDecisions(
		teamInstanceID,
		prepared,
	)
}

func (preparer *ProjectionMissionFallbackDecisionPreparer) prepare(
	candidate MissionFallbackDecisionCandidate,
	viewVersion string,
	expectedRevision int64,
) (PreparedMissionFallbackDecision, error) {
	scope := candidate.Scope
	revision := expectedRevision + 1
	approve := state.MissionFallbackDecisionCommand{
		CommandID: fmt.Sprintf(
			"fallback-approve-%s-r%d", scope.Digest()[:32], revision,
		),
		ExpectedRevision: expectedRevision,
		ActorRef:         preparer.actorRef,
		Scope:            scope,
		Decision:         state.MissionFallbackApproved,
	}
	reject := approve
	reject.CommandID = fmt.Sprintf(
		"fallback-reject-%s-r%d", scope.Digest()[:32], revision,
	)
	reject.Decision = state.MissionFallbackRejected
	sheet := MissionDecisionSheet{
		SchemaVersion: 1, Kind: "fallback",
		MissionID: candidate.MissionID, TeamInstanceID: scope.TeamInstanceID(),
		ViewVersion: viewVersion,
		DecisionID:  "fallback-" + scope.Digest()[:32],
		DecisionDigest: missionFallbackDecisionIntentDigest(
			scope, expectedRevision,
		),
		Title:     "Fallback route decision",
		Summary:   "Choose whether this Agent may use its configured fallback route.",
		Requester: "Loom", Target: candidate.AgentTitle,
		CommandType: fmt.Sprintf(
			"%s via %s with %s",
			candidate.HarnessAdapter,
			candidate.ProviderAccountID,
			candidate.ModelID,
		),
		NetworkAccess: fmt.Sprintf(
			"Provider %s through account %s",
			candidate.ProviderID,
			candidate.ProviderAccountID,
		),
		CredentialAccess: fmt.Sprintf(
			"Credential revision %d", candidate.CredentialRevision,
		),
		PermissionScope:  "This Agent and exact route only",
		AttemptScope:     "Attempt 2",
		ExpectedEvidence: "Versioned fallback decision in the Event Journal",
		TechnicalDetails: []string{
			"Scope " + scope.Digest()[:12],
			fmt.Sprintf("Decision revision %d", revision),
		},
		Actions: []string{
			"not_now", "reject_fallback", "approve_fallback",
		},
		PreparedActions: []string{"reject_fallback", "approve_fallback"},
		Prepared:        true, LogicalNodeID: scope.LogicalNodeID(),
		AttemptNumber: 2, ClaimGeneration: 0,
	}
	decision := PreparedMissionFallbackDecision{
		Sheet: sheet, Authority: preparer.authority,
		Approve: &approve, Reject: &reject,
		Refresh: MissionDecisionViewRefreshFunc(func(ctx context.Context) (string, error) {
			if ctx == nil {
				return "", ErrInvalidMissionDecision
			}
			if err := preparer.projection.Rebuild(ctx); err != nil {
				return "", err
			}
			return preparer.projection.GlobalReadView().Version(), nil
		}),
		Now: preparer.now,
	}
	if !validPreparedMissionFallbackDecision(decision) {
		return PreparedMissionFallbackDecision{}, errors.New(
			"invalid prepared Mission fallback decision",
		)
	}
	return decision, nil
}

func validMissionFallbackDecisionCandidate(
	candidate MissionFallbackDecisionCandidate,
) bool {
	return candidate.Scope.Valid() &&
		candidate.MissionID == "mission/"+candidate.Scope.TeamInstanceID() &&
		validDecisionSheetText(candidate.AgentTitle) &&
		validDecisionSheetText(candidate.HarnessAdapter) &&
		validDecisionSheetText(candidate.ProviderID) &&
		validDecisionSheetText(candidate.ProviderAccountID) &&
		validDecisionSheetText(candidate.ModelID) &&
		candidate.CredentialRevision > 0
}
