package projection

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
)

func TestTeamExecutionProjectionTracksLogicalAttemptsAndDeepCopies(t *testing.T) {
	digest := strings.Repeat("a", 64)
	viewVersion := strings.Repeat("b", 64)
	events := []journal.Event{
		teamProjectionEvent(t, "team-plan", "team-execution/team-1", 1, "team-plan", "TeamExecutionPlanned", map[string]any{
			"team_instance_id": "team-1",
			"plan_digest":      digest,
			"view_version":     viewVersion,
			"nodes": []map[string]any{{
				"logical_node_id":     "main",
				"title":               "Main",
				"agent_instance_id":   "agent-main",
				"runtime_instance_id": "runtime-main",
				"role":                "main",
				"depends_on":          []string{},
				"max_attempts":        2,
			}},
		}),
		teamProjectionEvent(t, "team-scheduled", "team-execution/team-1", 2, "team-scheduled", "TeamNodeAttemptScheduled", map[string]any{
			"logical_node_id":     "main",
			"attempt_number":      1,
			"work_item_id":        "team-work-1",
			"run_id":              "team-run-1",
			"runtime_instance_id": "runtime-main",
			"agent_instance_id":   "agent-main",
			"retry_at":            "",
		}),
		teamProjectionEvent(t, "team-dispatch", "team-execution/team-1", 3, "team-dispatch", "TeamReadySetDispatched", map[string]any{
			"team_instance_id": "team-1",
			"plan_digest":      digest,
			"view_version":     viewVersion,
			"attempts": []map[string]any{{
				"logical_node_id":     "main",
				"attempt_number":      1,
				"work_item_id":        "team-work-1",
				"run_id":              "team-run-1",
				"claim_id":            "11111111-1111-4111-8111-111111111111",
				"claim_generation":    1,
				"runtime_instance_id": "runtime-main",
				"agent_instance_id":   "agent-main",
			}},
		}),
		teamProjectionEvent(t, "team-rebound", "team-execution/team-1", 4, "team-rebound", "TeamNodeAttemptRebound", map[string]any{
			"team_instance_id":          "team-1",
			"plan_digest":               digest,
			"logical_node_id":           "main",
			"attempt_number":            1,
			"work_item_id":              "team-work-1",
			"run_id":                    "team-run-1",
			"previous_claim_id":         "11111111-1111-4111-8111-111111111111",
			"previous_claim_generation": 1,
			"claim_id":                  "22222222-2222-4222-8222-222222222222",
			"claim_generation":          2,
			"runtime_instance_id":       "runtime-main",
			"agent_instance_id":         "agent-main",
			"run_stream":                "run/team-run-1",
			"run_sequence":              2,
			"run_event_id":              "run-reclaim",
		}),
		teamProjectionEvent(t, "team-attempt-terminal", "team-execution/team-1", 5, "team-attempt-terminal", "TeamNodeAttemptTerminal", map[string]any{
			"logical_node_id":     "main",
			"attempt_number":      1,
			"work_item_id":        "team-work-1",
			"run_id":              "team-run-1",
			"claim_id":            "22222222-2222-4222-8222-222222222222",
			"claim_generation":    2,
			"runtime_instance_id": "runtime-main",
			"agent_instance_id":   "agent-main",
			"status":              "succeeded",
			"evidence_id":         "team-evidence-1",
			"evidence_digest":     digest,
		}),
		teamProjectionEvent(t, "team-terminal", "team-execution/team-1", 6, "team-terminal", "TeamExecutionTerminal", map[string]any{
			"team_instance_id": "team-1",
			"plan_digest":      digest,
			"status":           "succeeded",
			"reason":           "",
		}),
	}
	record, err := projectTeamExecutionStream(
		"team-1",
		events,
		map[string]teamExecutionRunClaimReference{
			teamExecutionRunReferenceKey("run/team-run-1", 2): {
				eventID: "run-reclaim",
			},
		},
	)
	if err != nil {
		t.Fatalf("projectTeamExecutionStream() error = %v", err)
	}
	view := buildGlobalReadView(
		emptySnapshot(),
		events,
		map[string]TeamExecution{"team-1": record},
	)
	record, ok := view.TeamExecution("team-1")
	if !ok || record.Status != "succeeded" || len(record.Nodes) != 1 ||
		len(record.Nodes[0].Attempts) != 1 {
		t.Fatalf("TeamExecution() = %#v, %v", record, ok)
	}
	want := TeamExecutionAttempt{
		AttemptNumber:     1,
		WorkItemID:        "team-work-1",
		RunID:             "team-run-1",
		ClaimID:           "22222222-2222-4222-8222-222222222222",
		ClaimGeneration:   2,
		RuntimeInstanceID: "runtime-main",
		AgentInstanceID:   "agent-main",
		Status:            "succeeded",
		EvidenceID:        "team-evidence-1",
		EvidenceDigest:    digest,
	}
	if got := record.Nodes[0].Attempts[0]; !reflect.DeepEqual(got, want) {
		t.Fatalf("attempt = %#v, want %#v", got, want)
	}
	record.Nodes[0].Attempts[0].EvidenceDigest = "mutated"
	again, _ := view.TeamExecution("team-1")
	if again.Nodes[0].Attempts[0].EvidenceDigest != digest {
		t.Fatal("Team execution view aliases caller mutation")
	}
}

func TestTeamExecutionProjectionStopsRecoveryAtMaxAttempts(t *testing.T) {
	digest := strings.Repeat("c", 64)
	viewVersion := strings.Repeat("d", 64)
	events := []journal.Event{
		teamProjectionEvent(t, "plan", "team-execution/team-max", 1, "plan", "TeamExecutionPlanned", map[string]any{
			"team_instance_id": "team-max",
			"plan_digest":      digest,
			"view_version":     viewVersion,
			"nodes": []map[string]any{{
				"logical_node_id":     "main",
				"title":               "Main",
				"agent_instance_id":   "agent-main",
				"runtime_instance_id": "runtime-main",
				"role":                "main",
				"depends_on":          []string{},
				"max_attempts":        1,
			}},
		}),
		teamProjectionEvent(t, "scheduled", "team-execution/team-max", 2, "scheduled", "TeamNodeAttemptScheduled", map[string]any{
			"logical_node_id":     "main",
			"attempt_number":      1,
			"work_item_id":        "team-work-max",
			"run_id":              "team-run-max",
			"runtime_instance_id": "runtime-main",
			"agent_instance_id":   "agent-main",
			"retry_at":            "",
		}),
		teamProjectionEvent(t, "dispatch", "team-execution/team-max", 3, "dispatch", "TeamReadySetDispatched", map[string]any{
			"team_instance_id": "team-max",
			"plan_digest":      digest,
			"view_version":     viewVersion,
			"attempts": []map[string]any{{
				"logical_node_id":     "main",
				"attempt_number":      1,
				"work_item_id":        "team-work-max",
				"run_id":              "team-run-max",
				"claim_id":            "11111111-1111-4111-8111-111111111111",
				"claim_generation":    1,
				"runtime_instance_id": "runtime-main",
				"agent_instance_id":   "agent-main",
			}},
		}),
		teamProjectionEvent(t, "terminal", "team-execution/team-max", 4, "terminal", "TeamNodeAttemptTerminal", map[string]any{
			"logical_node_id":     "main",
			"attempt_number":      1,
			"work_item_id":        "team-work-max",
			"run_id":              "team-run-max",
			"claim_id":            "11111111-1111-4111-8111-111111111111",
			"claim_generation":    1,
			"runtime_instance_id": "runtime-main",
			"agent_instance_id":   "agent-main",
			"status":              "failed",
			"evidence_id":         "team-evidence-max",
			"evidence_digest":     digest,
		}),
		teamProjectionEvent(t, "team-terminal", "team-execution/team-max", 5, "team-terminal", "TeamExecutionTerminal", map[string]any{
			"team_instance_id": "team-max",
			"plan_digest":      digest,
			"status":           "failed",
			"reason":           "node_main_failed",
		}),
	}
	readModel := newForTestSource(eventSliceSource{events: events})
	if err := readModel.Rebuild(context.Background()); err != nil {
		t.Fatalf("Rebuild() error = %v", err)
	}
	record, ok := readModel.GlobalReadView().TeamExecution("team-max")
	if !ok || record.Status != "failed" ||
		len(record.Nodes) != 1 || record.Nodes[0].Status != "failed" {
		t.Fatalf("terminal max-attempt projection = %#v, %v", record, ok)
	}
}

func teamProjectionEvent(
	t testing.TB,
	id string,
	streamID string,
	sequence int64,
	idempotencyKey string,
	eventType string,
	payload any,
) journal.Event {
	t.Helper()
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return journal.Event{
		ID: id, StreamID: streamID, Seq: sequence,
		IdempotencyKey: idempotencyKey, Type: eventType,
		SchemaVersion: 1,
		EmittedAt:     time.Date(2026, 7, 26, 1, 2, 3, 0, time.UTC),
		PayloadJSON:   encoded,
	}
}
