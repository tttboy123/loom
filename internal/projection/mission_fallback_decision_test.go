package projection

import (
	"context"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/state"
	"loom-pi-rebuild/internal/work"
)

func TestMissionFallbackDecisionProjectionReplaysLatestExactScope(t *testing.T) {
	database, store := openLocalProductSetupProjectionStore(t)
	writer, err := state.NewLocalProductSetupWriter(store)
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
	now := time.Date(2026, 8, 11, 8, 0, 0, 0, time.UTC)
	approved, err := writer.CommitMissionFallbackDecision(
		context.Background(), state.MissionFallbackDecisionCommand{
			CommandID: "approve-main-v1", ExpectedRevision: 0,
			OccurredAt: now, CorrelationID: "fallback-projection-1",
			ActorRef: "user:local-owner", Scope: scope,
			Decision: state.MissionFallbackApproved,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	readModel := New(database)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	record, ok := readModel.MissionFallbackDecision(scope)
	if !ok || record.Revision != 1 || record.Decision != "approved" ||
		record.ScopeDigest != scope.Digest() ||
		!record.Approval.Valid() ||
		record.Approval.Digest() != approved.Approval.Digest() {
		t.Fatalf("approved projection = %#v, ok=%t", record, ok)
	}
	drifted, err := work.NewTeamFallbackDecisionScope(
		work.TeamFallbackDecisionScopeInput{
			Version: 1, TeamInstanceID: scope.TeamInstanceID(),
			PlanDigest:          strings.Repeat("d", 64),
			LogicalNodeID:       scope.LogicalNodeID(),
			SourceBindingDigest: scope.SourceBindingDigest(),
			TargetBindingDigest: scope.TargetBindingDigest(),
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := readModel.MissionFallbackDecision(drifted); ok {
		t.Fatal("plan drift resolved a foreign fallback decision")
	}
	if _, err := writer.CommitMissionFallbackDecision(
		context.Background(), state.MissionFallbackDecisionCommand{
			CommandID: "reject-main-v2", ExpectedRevision: 1,
			OccurredAt: now.Add(time.Second), CorrelationID: "fallback-projection-2",
			ActorRef: "user:local-owner", Scope: scope,
			Decision: state.MissionFallbackRejected,
		},
	); err != nil {
		t.Fatal(err)
	}
	restarted := New(database)
	if err := restarted.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	rejected, ok := restarted.MissionFallbackDecision(scope)
	if !ok || rejected.Revision != 2 || rejected.Decision != "rejected" ||
		rejected.Approval.Valid() {
		t.Fatalf("rejected projection = %#v, ok=%t", rejected, ok)
	}
}
