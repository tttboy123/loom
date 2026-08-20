package api

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
)

func TestLocalProductMissionFacadeUsesRealProjectionAndPreservesStaleView(
	t *testing.T,
) {
	db := openAPITimelineDB(t)
	store := journal.NewStore(db)
	appendAPITimelineFixture(t, store)
	readModel := projection.New(db)
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	source := &controlledLocalProductViewSource{
		view: readModel.GlobalReadView(),
	}
	service, err := NewLocalProductReadService(LocalProductReadConfig{
		Journal:    store,
		Projection: source,
		Now: func() time.Time {
			return time.Date(2026, 7, 30, 8, 0, 0, 0, time.UTC)
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	snapshot, err := service.ReadLocalProductSnapshot(
		context.Background(),
		LocalProductSnapshotRequest{Limit: 64},
	)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.SchemaVersion != 3 || len(snapshot.Missions) != 1 {
		t.Fatalf("mission snapshot = %#v", snapshot)
	}
	mission := snapshot.Missions[0]
	if mission.MissionID != "mission/team-instance.one" ||
		mission.TeamInstanceID != "team-instance.one" ||
		mission.Title != "Saved team" ||
		mission.Lane != MissionLaneProposed ||
		mission.Status != "planned" ||
		mission.PlanDigest != strings.Repeat("a", 64) ||
		mission.NodeCount != 1 ||
		!mission.Simple ||
		len(mission.TeamPulse) != 1 {
		t.Fatalf("mission = %#v", mission)
	}
	for _, internalID := range []string{
		mission.MissionID, mission.TeamInstanceID, "team.delivery",
	} {
		if strings.Contains(mission.Title, internalID) {
			t.Fatalf("Mission title exposed internal ID %q: %#v", internalID, mission)
		}
	}
	pulse := mission.TeamPulse[0]
	if pulse.AgentInstanceID != "agent-instance.main" ||
		pulse.RuntimeInstanceID != "runtime.shared" ||
		pulse.Role != "main" ||
		pulse.NodeID != "main" {
		t.Fatalf("Team Pulse = %#v", pulse)
	}

	source.err = errors.New("private projection failure")
	stale, err := service.ReadLocalProductSnapshot(
		context.Background(),
		LocalProductSnapshotRequest{Limit: 64},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !stale.Stale ||
		stale.Reason != "projection_refresh_failed" ||
		!reflect.DeepEqual(stale.Missions, snapshot.Missions) {
		t.Fatalf("stale Missions = %#v, previous = %#v", stale, snapshot)
	}
}

func TestLocalProductMissionBlockReasonSurfacesTerminalFailure(t *testing.T) {
	execution := projection.TeamExecution{
		TeamInstanceID: "team-blocked-1",
		Status:         "running",
		Nodes: []projection.TeamExecutionNode{
			{LogicalNodeID: "main", Status: "pending"},
			{
				LogicalNodeID:  "subagent-1",
				Status:         "blocked",
				CurrentAttempt: 2,
				Attempts: []projection.TeamExecutionAttempt{
					{AttemptNumber: 1, TerminalReason: "provider_http"},
					{AttemptNumber: 2, TerminalReason: "context_retrieval_denied"},
				},
			},
		},
	}
	view := projection.GlobalReadView{}
	mission := buildLocalProductMission(view, execution)
	if mission.Status != "blocked" {
		t.Fatalf("mission status = %q, want blocked", mission.Status)
	}
	if mission.BlockReason != "context_retrieval_denied" {
		t.Fatalf("BlockReason = %q, want context_retrieval_denied", mission.BlockReason)
	}

	// Initial block reason wins over attempt terminal reasons.
	execution.Nodes[1].InitialBlockReason = "runtime_offline"
	execution.Nodes[1].InitialBlockCode = "runtime_offline"
	mission = buildLocalProductMission(view, execution)
	if mission.BlockReason != "runtime_offline" {
		t.Fatalf("BlockReason = %q, want runtime_offline", mission.BlockReason)
	}

	// A node the projection still marks "running" but whose current attempt
	// Run is terminal (the interrupted-daemon phantom case) surfaces the
	// terminal reason and reconciles the displayed status.
	execution.Nodes[1].Status = "running"
	if reason := localProductMissionBlockReason(view, execution); reason != "context_retrieval_denied" {
		t.Fatalf("BlockReason = %q, want context_retrieval_denied", reason)
	}
	if got := reconciledMissionStatus(view, execution); got != "context_retrieval_denied" &&
		got != "failed" && got != "running" {
		t.Fatalf("reconciled status = %q", got)
	}

	// awaiting_recovery nodes whose retry time passed without a new attempt
	// (pre-fix recovery records) surface the terminal reason too.
	execution.Nodes[1].Status = "awaiting_recovery"
	execution.Nodes[1].Attempts[1].TerminalReason = "provider_http"
	if reason := localProductMissionBlockReason(view, execution); reason != "provider_http" {
		t.Fatalf("awaiting_recovery BlockReason = %q, want provider_http", reason)
	}
}

func TestMissionLifecycleNeverCreatesAttentionLanes(t *testing.T) {
	for status, want := range map[string]MissionLane{
		"planned":             MissionLaneProposed,
		"ready":               MissionLaneReady,
		"running":             MissionLaneOrchestrating,
		"retrying":            MissionLaneOrchestrating,
		"blocked":             MissionLaneOrchestrating,
		"human_required":      MissionLaneOrchestrating,
		"ready_for_review":    MissionLaneReview,
		"awaiting_acceptance": MissionLaneReview,
		"succeeded":           MissionLaneComplete,
	} {
		if got := missionLaneForStatus(status, true); got != want {
			t.Fatalf("missionLaneForStatus(%q) = %q, want %q", status, got, want)
		}
	}
	for _, forbidden := range []MissionLane{"Draft", "Needs You", "Blocked", "Retrying"} {
		if validMissionLane(forbidden) {
			t.Fatalf("non-lifecycle lane accepted: %q", forbidden)
		}
	}
}
