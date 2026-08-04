// Package observability owns the v0.4.0 SF-W3 Timeline/Attention projection
// and authorized streaming node output. It is never an authority: frames are
// recorded as Journal facts only when authorized and generation-bound, and
// Timeline/Attention never triggers Agent authority.
package observability

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"loom-pi-rebuild/internal/integration"
	"loom-pi-rebuild/internal/journal"
)

var (
	ErrUnauthorized = errors.New("unauthorized streaming frame")
	ErrStaleFrame   = errors.New("stale generation frame")
	ErrMalformed    = errors.New("malformed streaming frame")
	ErrInvalidInput = errors.New("invalid observability input")
)

// FrameInput is the authorized, generation-bound node output frame.
type FrameInput struct {
	JourneyID   string `json:"journey_id"`
	OperationID string `json:"operation_id"`
	AttemptID   string `json:"attempt_id"`
	Generation  int64  `json:"generation"`
	NodeID      string `json:"node_id"`
	Kind        string `json:"kind"`
	Content     string `json:"content"`
	Authorized  bool   `json:"authorized"`
}

// ObservabilityService records and projects node output frames.
type ObservabilityService struct {
	store *journal.Store
	now   func() time.Time
}

func NewObservabilityService(store *journal.Store, now func() time.Time) (*ObservabilityService, error) {
	if store == nil || now == nil {
		return nil, ErrInvalidInput
	}
	return &ObservabilityService{store: store, now: now}, nil
}

// PublishFrame records a frame only when authorized and generation-bound;
// stale, unauthorized or malformed frames are rejected with zero effects.
func (service *ObservabilityService) PublishFrame(ctx context.Context, input FrameInput) ([]string, error) {
	if input.AttemptID == "" || input.OperationID == "" {
		return nil, ErrInvalidInput
	}
	if !input.Authorized {
		return nil, ErrUnauthorized
	}
	if input.Generation < 1 {
		return nil, ErrStaleFrame
	}
	if input.Content == "" || len(input.Content) > 4096 {
		return nil, ErrMalformed
	}
	now := service.now().UTC()
	frameSum := sha256.Sum256(
		[]byte("sf3\n" + input.AttemptID + "\n" + fmt.Sprint(input.Generation) + "\n" + input.OperationID))
	frame := integration.NodeOutputFrame{
		FrameID:   "frame-" + hex.EncodeToString(frameSum[:])[:20],
		AttemptID: input.AttemptID, Generation: input.Generation,
		NodeID: input.NodeID, Kind: input.Kind, Content: input.Content,
		Authorized: true, PublishedAt: now.Format(time.RFC3339Nano),
		CorrelationID: input.JourneyID,
	}
	stream := "observability/" + input.AttemptID
	data, err := json.Marshal(frame)
	if err != nil {
		return nil, ErrInvalidInput
	}
	seed := fmt.Sprintf("sf3\n%s\n%s\n%s\n%d", "NodeOutputFramePublished", stream, input.OperationID, 0)
	sum := sha256.Sum256([]byte(seed))
	event := journal.Event{
		ID:       "sf3-" + hex.EncodeToString(sum[:])[:32],
		StreamID: stream, Seq: 0,
		IdempotencyKey: fmt.Sprintf("sf3/%s/%s/%d", "NodeOutputFramePublished", input.OperationID, 0),
		Type:           "NodeOutputFramePublished", SchemaVersion: 1, EmittedAt: now,
		CorrelationID: input.JourneyID, PayloadJSON: data,
	}
	committed, err := service.append(ctx, []string{stream}, []journal.Event{event})
	if err != nil {
		return nil, err
	}
	return eventIDs(committed), nil
}

func (service *ObservabilityService) append(
	ctx context.Context,
	streams []string,
	events []journal.Event,
) ([]journal.Event, error) {
	snapshot, err := service.store.ReadStreamSet(ctx, streams)
	if err != nil {
		return nil, err
	}
	expectations := make([]journal.StreamHeadExpectation, 0, len(snapshot.Heads()))
	heads := make(map[string]int64, len(snapshot.Heads()))
	for _, head := range snapshot.Heads() {
		expectations = append(expectations, journal.StreamHeadExpectation{
			StreamID: head.StreamID, Sequence: head.Sequence,
		})
		heads[head.StreamID] = head.Sequence
	}
	counts := make(map[string]int, len(streams))
	for index := range events {
		stream := events[index].StreamID
		events[index].Seq = heads[stream] + int64(counts[stream]) + 1
		counts[stream]++
	}
	return service.store.AppendBatchIfStreamHeads(ctx, expectations, events)
}

func eventIDs(events []journal.Event) []string {
	ids := make([]string, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.ID)
	}
	return ids
}
