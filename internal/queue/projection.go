package queue

import (
	"encoding/json"
	"errors"
	"fmt"

	"loom-pi-rebuild/internal/journal"
)

var (
	ErrUnknownQueueEvent = errors.New("unknown queue event")
	ErrInvalidQueueEvent = errors.New("invalid queue event")
)

// Projection is the rebuildable queue read model. It is never an authority.
type Projection struct {
	Jobs       map[string]QueueJob
	Gaps       map[string]GapProposal
	Successors map[string]SuccessorProposal
}

func newProjection() *Projection {
	return &Projection{
		Jobs:       map[string]QueueJob{},
		Gaps:       map[string]GapProposal{},
		Successors: map[string]SuccessorProposal{},
	}
}

// Replay rebuilds the queue projection from ordered Journal events.
func Replay(events []journal.Event) (*Projection, error) {
	projection := newProjection()
	for _, event := range events {
		if err := apply(projection, event); err != nil {
			return nil, err
		}
	}
	return projection, nil
}

func apply(projection *Projection, event journal.Event) error {
	switch event.Type {
	case "QueueJobCreated":
		var job QueueJob
		if err := json.Unmarshal(event.PayloadJSON, &job); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidQueueEvent, err)
		}
		if job.JobID == "" {
			return fmt.Errorf("%w: empty job_id", ErrInvalidQueueEvent)
		}
		projection.Jobs[job.JobID] = job
	case "QueueJobAdmitted", "QueueJobCancelled":
		var transition struct {
			JobID  string    `json:"job_id"`
			Status JobStatus `json:"status"`
			Lane   Lane      `json:"lane"`
		}
		if err := json.Unmarshal(event.PayloadJSON, &transition); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidQueueEvent, err)
		}
		job, ok := projection.Jobs[transition.JobID]
		if !ok {
			return fmt.Errorf("%w: unknown job %q", ErrInvalidQueueEvent, transition.JobID)
		}
		job.Status = transition.Status
		if transition.Lane != "" {
			job.Lane = transition.Lane
		}
		projection.Jobs[transition.JobID] = job
	case "GapProposalCreated":
		var gap GapProposal
		if err := json.Unmarshal(event.PayloadJSON, &gap); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidQueueEvent, err)
		}
		if gap.GapID == "" {
			return fmt.Errorf("%w: empty gap_id", ErrInvalidQueueEvent)
		}
		projection.Gaps[gap.GapID] = gap
	case "SuccessorProposalCreated":
		var successor SuccessorProposal
		if err := json.Unmarshal(event.PayloadJSON, &successor); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidQueueEvent, err)
		}
		if successor.SuccessorProposalID == "" {
			return fmt.Errorf("%w: empty successor_proposal_id", ErrInvalidQueueEvent)
		}
		projection.Successors[successor.SuccessorProposalID] = successor
	default:
		// The Journal is shared with other authorities (identity index,
		// runtime discovery, evolution assets, execution, etc.). The queue
		// read model is a projection and must ignore unrelated event types
		// (same pattern as the accepted core projection); malformed queue
		// events above still fail closed.
		return nil
	}
	return nil
}
