package projection

import (
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/queue"
)

// QueueProjection is the separate rebuildable queue read model. It is never
// an authority; it is reconstructed from the Event Journal on every rebuild
// (restart/replay identical). It deliberately does not touch the accepted
// core Snapshot.
type QueueProjection struct {
	projection *queue.Projection
}

func RebuildQueueProjection(events []journal.Event) (*QueueProjection, error) {
	rebuilt, err := queue.Replay(events)
	if err != nil {
		return nil, err
	}
	return &QueueProjection{projection: rebuilt}, nil
}

func (projection *QueueProjection) Snapshot() *queue.Projection {
	if projection == nil {
		return nil
	}
	return cloneQueueProjection(projection.projection)
}

func cloneQueueProjection(input *queue.Projection) *queue.Projection {
	if input == nil {
		return nil
	}
	output := &queue.Projection{
		Jobs:       map[string]queue.QueueJob{},
		Gaps:       map[string]queue.GapProposal{},
		Successors: map[string]queue.SuccessorProposal{},
	}
	for key, record := range input.Jobs {
		record.Dependencies = append([]string(nil), record.Dependencies...)
		record.OwnedPaths = append([]string(nil), record.OwnedPaths...)
		record.MutexKeys = append([]string(nil), record.MutexKeys...)
		record.ExitConditions = append([]string(nil), record.ExitConditions...)
		record.ProtectedAuthorityPaths = append([]string(nil), record.ProtectedAuthorityPaths...)
		output.Jobs[key] = record
	}
	for key, record := range input.Gaps {
		record.Source.SourceIDs = append([]string(nil), record.Source.SourceIDs...)
		record.Source.SourceDigests = append([]string(nil), record.Source.SourceDigests...)
		record.OwnedPathClaims = append([]string(nil), record.OwnedPathClaims...)
		record.Supersedes = append([]string(nil), record.Supersedes...)
		output.Gaps[key] = record
	}
	for key, record := range input.Successors {
		record.SourceEvidenceDigests = append([]string(nil), record.SourceEvidenceDigests...)
		output.Successors[key] = record
	}
	return output
}
