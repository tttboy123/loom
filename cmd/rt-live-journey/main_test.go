package main

import (
	"testing"

	"loom-pi-rebuild/internal/roundtable"
)

func TestCloneSourceSeatsRequiresBoundExecutableSession(t *testing.T) {
	view := roundtable.View{
		Session: roundtable.Session{Context: &roundtable.SessionContext{
			ConversationID: "conversation-1", MissionID: "mission/team-1",
			TeamID: "team-1", TeamVersion: 1, WorkspaceID: "local",
		}},
		Seats: map[string]roundtable.Seat{
			"seat-main": {
				ID: "seat-main", Available: true,
				Binding: &roundtable.FrozenSeatBinding{AgentDefinitionID: "agent-main"},
			},
			"seat-peer": {
				ID: "seat-peer", Available: true,
				Binding: &roundtable.FrozenSeatBinding{AgentDefinitionID: "agent-peer"},
			},
		},
	}
	seats, err := cloneSourceSeats(view)
	if err != nil || len(seats) != 2 {
		t.Fatalf("cloneSourceSeats() = %#v, %v", seats, err)
	}
	view.Session.Context = nil
	if _, err := cloneSourceSeats(view); err == nil {
		t.Fatal("cloneSourceSeats() accepted an unbound session")
	}
}
