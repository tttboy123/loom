package projection

import (
	"encoding/json"
	"testing"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/queue"
)

func TestQueueProjectionRebuildsFromJournal(t *testing.T) {
	created := queue.QueueJob{
		JobID: "job-1", Source: "user_queued", DAGNodeID: "node-a",
		Status: queue.StatusQueued, Lane: queue.LaneAdmission,
		OwnedPaths: []string{"internal/queue/model.go"},
	}
	events := []journal.Event{
		queueEvent("QueueJobCreated", "queue-job/job-1", created),
		queueEvent("QueueJobAdmitted", "queue-job/job-1", struct {
			JobID  string          `json:"job_id"`
			Status queue.JobStatus `json:"status"`
			Lane   queue.Lane      `json:"lane"`
		}{JobID: "job-1", Status: queue.StatusAdmitted, Lane: queue.LaneDevelopment}),
	}
	first, err := RebuildQueueProjection(events)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RebuildQueueProjection(events)
	if err != nil {
		t.Fatal(err)
	}
	firstSnapshot := first.Snapshot()
	secondSnapshot := second.Snapshot()
	if firstSnapshot.Jobs["job-1"].Status != queue.StatusAdmitted {
		t.Fatalf("status = %s", firstSnapshot.Jobs["job-1"].Status)
	}
	if len(firstSnapshot.Jobs) != len(secondSnapshot.Jobs) {
		t.Fatalf("rebuild mismatch: %d vs %d", len(firstSnapshot.Jobs), len(secondSnapshot.Jobs))
	}
	firstSnapshot.Jobs["job-1"].OwnedPaths[0] = "mutated"
	if secondSnapshot.Jobs["job-1"].OwnedPaths[0] == "mutated" {
		t.Fatal("snapshot is not a deep copy")
	}
}

func queueEvent(eventType, streamID string, payload any) journal.Event {
	data, _ := json.Marshal(payload)
	return journal.Event{Type: eventType, StreamID: streamID, PayloadJSON: data}
}
