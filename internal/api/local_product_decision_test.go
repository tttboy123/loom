package api

import (
	"context"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/app"
)

type localProductDecisionBackend struct {
	last app.MissionDecisionCommand
}

func (backend *localProductDecisionBackend) ReadMissionDecision(
	_ context.Context,
	command app.MissionDecisionCommand,
) (app.MissionDecisionSheet, error) {
	backend.last = command
	return app.MissionDecisionSheet{
		SchemaVersion:    1,
		Kind:             command.Kind,
		MissionID:        command.MissionID,
		TeamInstanceID:   command.TeamInstanceID,
		ViewVersion:      command.ViewVersion,
		DecisionID:       command.DecisionID,
		DecisionDigest:   command.DecisionDigest,
		Title:            "Review",
		Summary:          "Prepared review",
		Requester:        "Reviewer",
		Target:           "Mission",
		CommandType:      "review",
		NetworkAccess:    "none",
		CredentialAccess: "none",
		PermissionScope:  "this Mission",
		AttemptScope:     "Attempt 1",
		ExpectedEvidence: "accepted Evidence",
		TechnicalDetails: []string{},
		Actions:          []string{"not_now", "request_changes", "accept_result"},
		PreparedActions:  []string{"accept_result"},
		Prepared:         true,
		LogicalNodeID:    command.LogicalNodeID,
		AttemptNumber:    command.AttemptNumber,
		ClaimGeneration:  command.ClaimGeneration,
	}, nil
}

func (backend *localProductDecisionBackend) DecideMission(
	_ context.Context,
	command app.MissionDecisionCommand,
) (app.MissionDecisionResult, error) {
	backend.last = command
	return app.MissionDecisionResult{
		SchemaVersion: 1,
		MissionID:     command.MissionID,
		DecisionID:    command.DecisionID,
		Status:        "approved",
		Authoritative: true,
		ViewVersion:   strings.Repeat("c", 64),
	}, nil
}

func TestLocalProductDecisionAPICanonicalizesExactCommandAndResult(t *testing.T) {
	backend := &localProductDecisionBackend{}
	service, err := NewLocalProductDecisionAPI(backend)
	if err != nil {
		t.Fatal(err)
	}
	command := app.MissionDecisionCommand{
		SchemaVersion:   1,
		Operation:       "submit",
		Kind:            "authorization",
		Action:          "allow_once",
		MissionID:       "mission/team-1",
		TeamInstanceID:  "team-1",
		ViewVersion:     strings.Repeat("a", 64),
		DecisionID:      "approval-1",
		DecisionDigest:  strings.Repeat("b", 64),
		LogicalNodeID:   "main",
		AttemptNumber:   1,
		ClaimGeneration: 1,
		CorrelationID:   "11111111-1111-4111-8111-111111111111",
	}
	result, err := service.DecideMission(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if backend.last != command ||
		result.Status != "approved" ||
		!result.Authoritative ||
		result.ViewVersion != strings.Repeat("c", 64) {
		t.Fatalf("last=%#v result=%#v", backend.last, result)
	}
}
