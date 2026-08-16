package state

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/work"
)

var (
	ErrInvalidMissionFallbackDecision = errors.New(
		"invalid Mission fallback decision",
	)
	ErrMissionFallbackDecisionConflict = errors.New(
		"Mission fallback decision conflict",
	)
)

type MissionFallbackDecision string

const (
	MissionFallbackApproved MissionFallbackDecision = "approved"
	MissionFallbackRejected MissionFallbackDecision = "rejected"
)

type MissionFallbackDecisionCommand struct {
	CommandID        string
	ExpectedRevision int64
	OccurredAt       time.Time
	CorrelationID    string
	ActorRef         string
	Scope            work.TeamFallbackDecisionScope
	Decision         MissionFallbackDecision
}

type MissionFallbackDecisionResult struct {
	ScopeDigest string
	Revision    int64
	Decision    MissionFallbackDecision
	Approval    work.TeamFallbackApproval
}

type missionFallbackApprovalPayload struct {
	Version             int    `json:"version"`
	ApprovalID          string `json:"approval_id"`
	ActorRef            string `json:"actor_ref"`
	ApprovedAt          string `json:"approved_at"`
	SourceBindingDigest string `json:"source_binding_digest"`
	TargetBindingDigest string `json:"target_binding_digest"`
	Digest              string `json:"digest"`
}

type missionFallbackDecisionPayload struct {
	ScopeVersion        int                             `json:"scope_version"`
	TeamInstanceID      string                          `json:"team_instance_id"`
	PlanDigest          string                          `json:"plan_digest"`
	LogicalNodeID       string                          `json:"logical_node_id"`
	SourceBindingDigest string                          `json:"source_binding_digest"`
	TargetBindingDigest string                          `json:"target_binding_digest"`
	ScopeDigest         string                          `json:"scope_digest"`
	Revision            int64                           `json:"revision"`
	Decision            MissionFallbackDecision         `json:"decision"`
	ActorRef            string                          `json:"actor_ref"`
	DecidedAt           string                          `json:"decided_at"`
	Approval            *missionFallbackApprovalPayload `json:"approval"`
}

func (writer *LocalProductSetupWriter) CommitMissionFallbackDecision(
	ctx context.Context,
	command MissionFallbackDecisionCommand,
) (MissionFallbackDecisionResult, error) {
	if writer == nil || writer.store == nil || ctx == nil ||
		!validSetupIdentifier(command.CommandID, 128) ||
		command.ExpectedRevision < 0 || command.ExpectedRevision >= 1_000_000 ||
		!validSetupTime(command.OccurredAt) ||
		!validSetupIdentifier(command.CorrelationID, 128) ||
		!validSetupIdentifier(command.ActorRef, 128) ||
		!command.Scope.Valid() ||
		(command.Decision != MissionFallbackApproved &&
			command.Decision != MissionFallbackRejected) {
		return MissionFallbackDecisionResult{}, ErrInvalidMissionFallbackDecision
	}
	revision := command.ExpectedRevision + 1
	approval := work.TeamFallbackApproval{}
	if command.Decision == MissionFallbackApproved {
		var err error
		approval, err = work.NewTeamFallbackApproval(work.TeamFallbackApprovalInput{
			Version: int(revision),
			ApprovalID: missionFallbackApprovalID(
				command.Scope.Digest(), revision,
			),
			ActorRef: command.ActorRef, ApprovedAt: command.OccurredAt,
			SourceBindingDigest: command.Scope.SourceBindingDigest(),
			TargetBindingDigest: command.Scope.TargetBindingDigest(),
		})
		if err != nil {
			return MissionFallbackDecisionResult{}, ErrInvalidMissionFallbackDecision
		}
	}
	payload := missionFallbackDecisionPayload{
		ScopeVersion:        command.Scope.Version(),
		TeamInstanceID:      command.Scope.TeamInstanceID(),
		PlanDigest:          command.Scope.PlanDigest(),
		LogicalNodeID:       command.Scope.LogicalNodeID(),
		SourceBindingDigest: command.Scope.SourceBindingDigest(),
		TargetBindingDigest: command.Scope.TargetBindingDigest(),
		ScopeDigest:         command.Scope.Digest(), Revision: revision,
		Decision: command.Decision, ActorRef: command.ActorRef,
		DecidedAt: command.OccurredAt.Format(time.RFC3339Nano),
	}
	eventType := "MissionFallbackRejected"
	if approval.Valid() {
		eventType = "MissionFallbackApproved"
		payload.Approval = &missionFallbackApprovalPayload{
			Version: approval.Version(), ApprovalID: approval.ApprovalID(),
			ActorRef:            approval.ActorRef(),
			ApprovedAt:          approval.ApprovedAt().Format(time.RFC3339Nano),
			SourceBindingDigest: approval.SourceBindingDigest(),
			TargetBindingDigest: approval.TargetBindingDigest(),
			Digest:              approval.Digest(),
		}
	}
	streamID := MissionFallbackDecisionStreamID(command.Scope)
	event, err := setupEvent(
		command.CommandID, streamID, revision, eventType,
		command.OccurredAt, command.CorrelationID, "", payload,
	)
	if err != nil {
		return MissionFallbackDecisionResult{}, ErrInvalidMissionFallbackDecision
	}
	if _, err := writer.store.AppendBatchIfStreamHeads(
		ctx,
		[]journal.StreamHeadExpectation{{
			StreamID: streamID, Sequence: command.ExpectedRevision,
		}},
		[]journal.Event{event},
	); err != nil {
		if errors.Is(err, journal.ErrStreamHeadConflict) ||
			errors.Is(err, journal.ErrSequenceConflict) ||
			errors.Is(err, journal.ErrIdempotencyConflict) {
			return MissionFallbackDecisionResult{},
				errors.Join(ErrMissionFallbackDecisionConflict, err)
		}
		return MissionFallbackDecisionResult{}, err
	}
	return MissionFallbackDecisionResult{
		ScopeDigest: command.Scope.Digest(), Revision: revision,
		Decision: command.Decision, Approval: approval,
	}, nil
}

func MissionFallbackDecisionStreamID(
	scope work.TeamFallbackDecisionScope,
) string {
	if !scope.Valid() {
		return ""
	}
	return "mission-fallback-decision/" + scope.Digest()
}

func missionFallbackApprovalID(scopeDigest string, revision int64) string {
	sum := sha256.Sum256([]byte(
		fmt.Sprintf("loom.mission-fallback-approval.v1\x00%s\x00%d", scopeDigest, revision),
	))
	return "fallback-approval-" + hex.EncodeToString(sum[:16])
}
