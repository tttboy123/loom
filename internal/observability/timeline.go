package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"loom-pi-rebuild/internal/integration"
	"loom-pi-rebuild/internal/journal"
)

// Timeline is the rebuildable read model of streaming node output.
type Timeline struct {
	Frames []integration.NodeOutputFrame
}

// BuildTimeline reconstructs the timeline from Journal frames.
func BuildTimeline(ctx context.Context, store *journal.Store) (*Timeline, error) {
	events, err := store.ReadAll(ctx)
	if err != nil {
		return nil, err
	}
	var frames []integration.NodeOutputFrame
	for _, event := range events {
		if event.Type != "NodeOutputFramePublished" {
			continue
		}
		var frame integration.NodeOutputFrame
		if err := json.Unmarshal(event.PayloadJSON, &frame); err != nil {
			return nil, fmt.Errorf("timeline malformed frame: %w", err)
		}
		frames = append(frames, frame)
	}
	sort.Slice(frames, func(i, j int) bool {
		return frames[i].PublishedAt < frames[j].PublishedAt
	})
	return &Timeline{Frames: frames}, nil
}
