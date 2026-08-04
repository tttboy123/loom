package observability

import (
	"context"
	"encoding/json"
	"fmt"

	"loom-pi-rebuild/internal/integration"
	"loom-pi-rebuild/internal/journal"
)

// AttentionItem is a needs-your-attention projection item (never an
// authority; it only reflects Journal facts).
type AttentionItem struct {
	Kind     string `json:"kind"`
	SourceID string `json:"source_id"`
	Summary  string `json:"summary"`
	Lane     string `json:"lane"`
}

// BuildAttention reconstructs the attention list from Journal
// human_required frames.
func BuildAttention(ctx context.Context, store *journal.Store) ([]AttentionItem, error) {
	events, err := store.ReadAll(ctx)
	if err != nil {
		return nil, err
	}
	var items []AttentionItem
	seen := map[string]bool{}
	for _, event := range events {
		if event.Type != "NodeOutputFramePublished" {
			continue
		}
		var frame integration.NodeOutputFrame
		if err := json.Unmarshal(event.PayloadJSON, &frame); err != nil {
			return nil, fmt.Errorf("attention malformed frame: %w", err)
		}
		if frame.Kind != "human_required" || seen[frame.AttemptID] {
			continue
		}
		seen[frame.AttemptID] = true
		items = append(items, AttentionItem{
			Kind: "human_required", SourceID: frame.AttemptID,
			Summary: frame.Content, Lane: "human",
		})
	}
	return items, nil
}
