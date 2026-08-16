package projection

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/work"
)

type MissionFallbackDecisionRecord struct {
	Scope       work.TeamFallbackDecisionScope
	ScopeDigest string
	Revision    int64
	Decision    string
	ActorRef    string
	DecidedAt   time.Time
	Approval    work.TeamFallbackApproval
}

type projectedMissionFallbackApprovalPayload struct {
	Version             int    `json:"version"`
	ApprovalID          string `json:"approval_id"`
	ActorRef            string `json:"actor_ref"`
	ApprovedAt          string `json:"approved_at"`
	SourceBindingDigest string `json:"source_binding_digest"`
	TargetBindingDigest string `json:"target_binding_digest"`
	Digest              string `json:"digest"`
}

type projectedMissionFallbackDecisionPayload struct {
	ScopeVersion        int                                      `json:"scope_version"`
	TeamInstanceID      string                                   `json:"team_instance_id"`
	PlanDigest          string                                   `json:"plan_digest"`
	LogicalNodeID       string                                   `json:"logical_node_id"`
	SourceBindingDigest string                                   `json:"source_binding_digest"`
	TargetBindingDigest string                                   `json:"target_binding_digest"`
	ScopeDigest         string                                   `json:"scope_digest"`
	Revision            int64                                    `json:"revision"`
	Decision            string                                   `json:"decision"`
	ActorRef            string                                   `json:"actor_ref"`
	DecidedAt           string                                   `json:"decided_at"`
	Approval            *projectedMissionFallbackApprovalPayload `json:"approval"`
}

func (p *Projection) MissionFallbackDecision(
	scope work.TeamFallbackDecisionScope,
) (MissionFallbackDecisionRecord, bool) {
	if p == nil || !scope.Valid() {
		return MissionFallbackDecisionRecord{}, false
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	record, ok := p.snapshot.MissionFallbackDecisions[scope.Digest()]
	if !ok || record.ScopeDigest != scope.Digest() ||
		!record.Scope.Valid() {
		return MissionFallbackDecisionRecord{}, false
	}
	return record, true
}

func isMissionFallbackDecisionProjectionEvent(event journal.Event) bool {
	return event.Type == "MissionFallbackApproved" ||
		event.Type == "MissionFallbackRejected"
}

func applyMissionFallbackDecisionProjection(
	snapshot *Snapshot,
	event journal.Event,
) error {
	if snapshot == nil || event.Seq <= 0 || event.SchemaVersion != 1 {
		return ErrInvalidProjectionEvent
	}
	var payload projectedMissionFallbackDecisionPayload
	decoder := json.NewDecoder(bytes.NewReader(event.PayloadJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return errors.Join(ErrInvalidProjectionEvent, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return ErrInvalidProjectionEvent
	}
	scope, err := work.NewTeamFallbackDecisionScope(
		work.TeamFallbackDecisionScopeInput{
			Version:             payload.ScopeVersion,
			TeamInstanceID:      payload.TeamInstanceID,
			PlanDigest:          payload.PlanDigest,
			LogicalNodeID:       payload.LogicalNodeID,
			SourceBindingDigest: payload.SourceBindingDigest,
			TargetBindingDigest: payload.TargetBindingDigest,
		},
	)
	decidedAt, timeErr := time.Parse(time.RFC3339Nano, payload.DecidedAt)
	if err != nil || timeErr != nil ||
		payload.ScopeDigest != scope.Digest() ||
		payload.Revision != event.Seq ||
		event.StreamID != "mission-fallback-decision/"+scope.Digest() ||
		!event.EmittedAt.Equal(decidedAt) {
		return ErrInvalidProjectionEvent
	}
	validation, validationErr := work.NewTeamFallbackApproval(
		work.TeamFallbackApprovalInput{
			Version: 1, ApprovalID: "fallback-validation",
			ActorRef: payload.ActorRef, ApprovedAt: decidedAt,
			SourceBindingDigest: scope.SourceBindingDigest(),
			TargetBindingDigest: scope.TargetBindingDigest(),
		},
	)
	if validationErr != nil || !validation.Valid() {
		return ErrInvalidProjectionEvent
	}
	approval := work.TeamFallbackApproval{}
	switch payload.Decision {
	case "approved":
		if event.Type != "MissionFallbackApproved" || payload.Approval == nil {
			return ErrInvalidProjectionEvent
		}
		approvedAt, parseErr := time.Parse(
			time.RFC3339Nano, payload.Approval.ApprovedAt,
		)
		if parseErr != nil {
			return ErrInvalidProjectionEvent
		}
		approval, err = work.NewTeamFallbackApproval(
			work.TeamFallbackApprovalInput{
				Version:             payload.Approval.Version,
				ApprovalID:          payload.Approval.ApprovalID,
				ActorRef:            payload.Approval.ActorRef,
				ApprovedAt:          approvedAt,
				SourceBindingDigest: payload.Approval.SourceBindingDigest,
				TargetBindingDigest: payload.Approval.TargetBindingDigest,
			},
		)
		if err != nil || approval.Digest() != payload.Approval.Digest ||
			approval.Version() != int(payload.Revision) ||
			approval.ActorRef() != payload.ActorRef ||
			!approval.ApprovedAt().Equal(decidedAt) ||
			approval.SourceBindingDigest() != scope.SourceBindingDigest() ||
			approval.TargetBindingDigest() != scope.TargetBindingDigest() {
			return ErrInvalidProjectionEvent
		}
	case "rejected":
		if event.Type != "MissionFallbackRejected" || payload.Approval != nil {
			return ErrInvalidProjectionEvent
		}
	default:
		return ErrInvalidProjectionEvent
	}
	if snapshot.MissionFallbackDecisions == nil {
		snapshot.MissionFallbackDecisions = make(
			map[string]MissionFallbackDecisionRecord,
		)
	}
	if prior, ok := snapshot.MissionFallbackDecisions[scope.Digest()]; ok &&
		prior.Revision+1 != payload.Revision {
		return fmt.Errorf("%w: fallback decision revision", ErrInvalidProjectionEvent)
	}
	snapshot.MissionFallbackDecisions[scope.Digest()] = MissionFallbackDecisionRecord{
		Scope: scope, ScopeDigest: scope.Digest(), Revision: payload.Revision,
		Decision: payload.Decision, ActorRef: payload.ActorRef,
		DecidedAt: decidedAt, Approval: approval,
	}
	return nil
}
