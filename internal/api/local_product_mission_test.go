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
	if snapshot.SchemaVersion != 2 || len(snapshot.Missions) != 1 {
		t.Fatalf("mission snapshot = %#v", snapshot)
	}
	mission := snapshot.Missions[0]
	if mission.MissionID != "mission/team-instance.one" ||
		mission.TeamInstanceID != "team-instance.one" ||
		mission.Lane != MissionLaneProposed ||
		mission.Status != "planned" ||
		mission.PlanDigest != strings.Repeat("a", 64) ||
		mission.NodeCount != 1 ||
		!mission.Simple ||
		len(mission.TeamPulse) != 1 {
		t.Fatalf("mission = %#v", mission)
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
