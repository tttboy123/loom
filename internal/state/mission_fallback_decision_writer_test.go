package state

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/work"
)

func TestMissionFallbackDecisionWriterPersistsVersionedApproveAndReject(t *testing.T) {
	store := openLocalProductSetupWriterStore(t)
	writer, err := NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := work.NewTeamFallbackDecisionScope(
		work.TeamFallbackDecisionScopeInput{
			Version: 1, TeamInstanceID: "team-mixed-instance",
			PlanDigest: strings.Repeat("a", 64), LogicalNodeID: "main",
			SourceBindingDigest: strings.Repeat("b", 64),
			TargetBindingDigest: strings.Repeat("c", 64),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 11, 7, 0, 0, 0, time.UTC)
	approved, err := writer.CommitMissionFallbackDecision(
		context.Background(), MissionFallbackDecisionCommand{
			CommandID: "approve-fallback-main-v1", ExpectedRevision: 0,
			OccurredAt: now, CorrelationID: "fallback-incident-1",
			ActorRef: "user:local-owner", Scope: scope,
			Decision: MissionFallbackApproved,
		},
	)
	if err != nil || approved.Revision != 1 ||
		approved.Decision != MissionFallbackApproved ||
		!approved.Approval.Valid() ||
		approved.Approval.SourceBindingDigest() != scope.SourceBindingDigest() ||
		approved.Approval.TargetBindingDigest() != scope.TargetBindingDigest() {
		t.Fatalf("approved = %#v, err=%v", approved, err)
	}
	if _, err := writer.CommitMissionFallbackDecision(
		context.Background(), MissionFallbackDecisionCommand{
			CommandID: "stale-fallback-main", ExpectedRevision: 0,
			OccurredAt: now.Add(time.Second), CorrelationID: "fallback-incident-stale",
			ActorRef: "user:local-owner", Scope: scope,
			Decision: MissionFallbackRejected,
		},
	); !errors.Is(err, ErrMissionFallbackDecisionConflict) {
		t.Fatalf("stale decision error = %v", err)
	}
	rejected, err := writer.CommitMissionFallbackDecision(
		context.Background(), MissionFallbackDecisionCommand{
			CommandID: "reject-fallback-main-v2", ExpectedRevision: 1,
			OccurredAt: now.Add(2 * time.Second), CorrelationID: "fallback-incident-2",
			ActorRef: "user:local-owner", Scope: scope,
			Decision: MissionFallbackRejected,
		},
	)
	if err != nil || rejected.Revision != 2 ||
		rejected.Decision != MissionFallbackRejected ||
		rejected.Approval.Valid() {
		t.Fatalf("rejected = %#v, err=%v", rejected, err)
	}
	events, err := store.ReadStream(
		context.Background(), MissionFallbackDecisionStreamID(scope),
	)
	if err != nil || len(events) != 2 ||
		events[0].Type != "MissionFallbackApproved" ||
		events[1].Type != "MissionFallbackRejected" {
		t.Fatalf("events = %#v, err=%v", events, err)
	}
	for _, event := range events {
		var payload map[string]any
		if err := json.Unmarshal(event.PayloadJSON, &payload); err != nil {
			t.Fatal(err)
		}
		encoded := string(event.PayloadJSON)
		for _, forbidden := range []string{
			"credential_reference", "api_key", "authorization", "prompt",
		} {
			if strings.Contains(strings.ToLower(encoded), forbidden) {
				t.Fatalf("event %s leaked forbidden field %q: %s", event.ID, forbidden, encoded)
			}
		}
	}
}

func TestMissionFallbackDecisionWriterRejectsInvalidScope(t *testing.T) {
	store := openLocalProductSetupWriterStore(t)
	writer, err := NewLocalProductSetupWriter(store)
	if err != nil {
		t.Fatal(err)
	}
	_, err = writer.CommitMissionFallbackDecision(
		context.Background(), MissionFallbackDecisionCommand{
			CommandID: "approve-invalid-fallback", ExpectedRevision: 0,
			OccurredAt: time.Now().UTC(), CorrelationID: "fallback-invalid",
			ActorRef: "user:local-owner", Decision: MissionFallbackApproved,
		},
	)
	if !errors.Is(err, ErrInvalidMissionFallbackDecision) {
		t.Fatalf("invalid scope error = %v", err)
	}
	events, readErr := store.ReadStream(context.Background(), "mission-fallback-decision/invalid")
	if readErr != nil || len(events) != 0 {
		t.Fatalf("invalid stream read = %#v, %v", events, readErr)
	}
}
