package queue

import (
	"encoding/json"
	"testing"

	"loom-pi-rebuild/internal/journal"
)

func TestReplayRebuildsQueueProjectionFromJournal(t *testing.T) {
	created := QueueJob{
		JobID:  "123e4567-e89b-42d3-a456-426614174000",
		Source: "user_queued", DAGNodeID: "node-a",
		Status: StatusQueued, Lane: LaneAdmission,
		OwnedPaths:  []string{"internal/queue/model.go"},
		MaxAttempts: 3, CapabilityKind: "feature",
		ExitConditions: []string{"focused tests pass"},
	}
	events := []journal.Event{
		queueJobEvent("QueueJobCreated", created.JobID, created),
		queueJobTransitionEvent("QueueJobAdmitted", created.JobID, StatusAdmitted, LaneDevelopment),
	}
	first, err := Replay(events)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Replay(events)
	if err != nil {
		t.Fatal(err)
	}
	job := first.Jobs[created.JobID]
	if job.Status != StatusAdmitted {
		t.Fatalf("status = %s, want %s", job.Status, StatusAdmitted)
	}
	if job.Lane != LaneDevelopment {
		t.Fatalf("lane = %s, want %s", job.Lane, LaneDevelopment)
	}
	if second.Jobs[created.JobID].Status != job.Status {
		t.Fatalf("journal rebuild differs: %+v vs %+v", second.Jobs[created.JobID], job)
	}
}

func TestReplayCancelledIsTerminal(t *testing.T) {
	jobID := "123e4567-e89b-42d3-a456-426614174000"
	created := QueueJob{JobID: jobID, Source: "user_queued", DAGNodeID: "n",
		Status: StatusQueued, Lane: LaneAdmission, OwnedPaths: []string{"x"}}
	events := []journal.Event{
		queueJobEvent("QueueJobCreated", jobID, created),
		queueJobTransitionEvent("QueueJobCancelled", jobID, StatusCancelled, LaneHuman),
	}
	projection, err := Replay(events)
	if err != nil {
		t.Fatal(err)
	}
	if projection.Jobs[jobID].Status != StatusCancelled {
		t.Fatalf("status = %s, want terminal cancelled", projection.Jobs[jobID].Status)
	}
}

func TestReplaySkipsUnrelatedJournalEvents(t *testing.T) {
	events := []journal.Event{
		{
			Type: "QueueJobMystery", StreamID: "job-1",
			PayloadJSON: []byte(`{}`),
		},
		{
			// Unrelated authority events share the Journal (identity index,
			// runtime discovery, evolution assets, execution) and must be
			// ignored by the queue read model, exactly like the accepted core
			// projection.
			Type: "RuntimeInstanceDiscovered", StreamID: "runtime-1",
			PayloadJSON: []byte(`{}`),
		},
	}
	projection, err := Replay(events)
	if err != nil {
		t.Fatalf("Replay rejected unrelated journal events: %v", err)
	}
	if len(projection.Jobs) != 0 || len(projection.Gaps) != 0 ||
		len(projection.Successors) != 0 {
		t.Fatalf("unrelated events must not create queue facts: %+v", projection)
	}
}

func TestReplayRejectsMalformedQueueEvent(t *testing.T) {
	events := []journal.Event{
		{Type: "QueueJobCreated", StreamID: "job-1", PayloadJSON: []byte(`{"job_id":""}`)},
	}
	if _, err := Replay(events); err == nil {
		t.Fatal("Replay accepted malformed queue event, want error")
	}
}

func queueJobEvent(eventType, streamID string, payload any) journal.Event {
	data, _ := json.Marshal(payload)
	return journal.Event{
		Type: eventType, StreamID: streamID, PayloadJSON: data,
	}
}

func queueJobTransitionEvent(eventType, jobID string, status JobStatus, lane Lane) journal.Event {
	data, _ := json.Marshal(struct {
		JobID  string    `json:"job_id"`
		Status JobStatus `json:"status"`
		Lane   Lane      `json:"lane"`
	}{JobID: jobID, Status: status, Lane: lane})
	return journal.Event{Type: eventType, StreamID: jobID, PayloadJSON: data}
}
