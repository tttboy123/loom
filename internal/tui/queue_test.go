package tui

import (
	"context"
	"strings"
	"testing"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/queue"
)

type stubQueueClient struct {
	snapshot api.QueueSnapshot
	result   api.QueueCommandResult
}

func (client *stubQueueClient) QueueSnapshot(
	context.Context,
	api.QueueSnapshotRequest,
) (api.QueueSnapshot, error) {
	return client.snapshot, nil
}

func (client *stubQueueClient) QueueCommand(
	context.Context,
	api.QueueCommandRequest,
) (api.QueueCommandResult, error) {
	return client.result, nil
}

func TestQueueScreenRendersJobsGapsAndSuccessors(t *testing.T) {
	client := &stubQueueClient{
		snapshot: api.QueueSnapshot{
			ViewVersion: "v1",
			Jobs: []queue.QueueJob{
				{
					JobID: "job-a", Status: queue.StatusAdmitted,
					Lane: queue.LaneDevelopment, DAGNodeID: "node-a",
					OwnedPaths: []string{"internal/queue/model.go"},
				},
			},
			Gaps: []queue.GapProposal{
				{GapID: "gap-1", AffectedCapability: "scheduling", Disposition: "observe"},
			},
			Successors: []queue.SuccessorProposal{
				{SuccessorProposalID: "spr-1", GapID: "gap-1"},
			},
		},
	}
	model, err := newModelWithContext(context.Background(), &stubReadClient{queue: client})
	if err != nil {
		t.Fatal(err)
	}
	model.queueClient = client
	model.queueSnapshot = client.snapshot
	model.screenIndex = indexOfScreen(ScreenQueue)
	body := model.renderQueueView()
	for _, want := range []string{
		"Development Queue", "job-a", "Admitted", "Development", "node-a",
		"internal/queue/model.go", "gap-1", "scheduling", "spr-1",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("queue view missing %q in:\n%s", want, body)
		}
	}
}

type stubReadClient struct {
	queue QueueClient
}

func (client *stubReadClient) Snapshot(
	context.Context,
	api.LocalProductSnapshotRequest,
) (api.LocalProductSnapshot, error) {
	return api.LocalProductSnapshot{}, nil
}

func (client *stubReadClient) TimelinePage(
	context.Context,
	api.LocalProductTimelineRequest,
) (api.LocalProductTimelinePage, error) {
	return api.LocalProductTimelinePage{}, nil
}

func (client *stubReadClient) QueueSnapshot(
	ctx context.Context,
	request api.QueueSnapshotRequest,
) (api.QueueSnapshot, error) {
	return client.queue.QueueSnapshot(ctx, request)
}

func (client *stubReadClient) QueueCommand(
	ctx context.Context,
	request api.QueueCommandRequest,
) (api.QueueCommandResult, error) {
	return client.queue.QueueCommand(ctx, request)
}
