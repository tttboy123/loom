package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"loom-pi-rebuild/internal/journal"
)

// IntegrationService is the authoritative single-writer Integrator: one
// reviewed Candidate is integrated per CAS; competing/stale integrations
// lose with zero effects.
type IntegrationService struct {
	store *journal.Store
	now   func() time.Time
}

func NewIntegrationService(store *journal.Store, now func() time.Time) (*IntegrationService, error) {
	if store == nil || now == nil {
		return nil, ErrInvalidInput
	}
	return &IntegrationService{store: store, now: now}, nil
}

type IntegrateInput struct {
	JourneyID         string   `json:"journey_id"`
	OperationID       string   `json:"operation_id"`
	CandidateID       string   `json:"candidate_id"`
	TargetBranch      string   `json:"target_branch"`
	BaseCommit        string   `json:"base_commit"`
	SourceDigest      string   `json:"source_digest"`
	EvidenceDigest    string   `json:"evidence_digest"`
	DependencyDigests []string `json:"dependency_digests"`
}

type IntegrationResult struct {
	OperationID string   `json:"operation_id"`
	Action      string   `json:"action"`
	EventIDs    []string `json:"event_ids"`
	ReleaseID   string   `json:"release_id,omitempty"`
	Disposition string   `json:"disposition"`
}

// Integrate applies one reviewed Candidate. The integration stream CAS
// guarantees exactly one winner.
func (service *IntegrationService) Integrate(ctx context.Context, input IntegrateInput) (IntegrationResult, error) {
	if input.CandidateID == "" || input.TargetBranch == "" || input.OperationID == "" {
		return IntegrationResult{}, ErrInvalidInput
	}
	now := service.now().UTC()
	stream := "integration/" + input.TargetBranch
	existing, err := service.store.ReadStream(context.Background(), stream)
	if err != nil {
		return IntegrationResult{}, err
	}
	for _, event := range existing {
		if event.Type == "CandidateIntegrated" || event.Type == "IntegrationStarted" {
			return IntegrationResult{}, ErrStaleIntegration
		}
	}
	started, err := service.buildEvents(
		"IntegrationStarted", stream, input.OperationID, now, input.JourneyID,
		map[string]any{
			"candidate_id": input.CandidateID, "target_branch": input.TargetBranch,
			"base_commit": input.BaseCommit,
		},
	)
	if err != nil {
		return IntegrationResult{}, err
	}
	releaseSum := sha256.Sum256(
		[]byte("sf3\n" + input.CandidateID + "\n" + input.TargetBranch + "\n" + input.SourceDigest))
	releaseID := "release-" + hex.EncodeToString(releaseSum[:])[:24]
	release := ReleaseCandidate{
		ReleaseID: releaseID, CandidateID: input.CandidateID,
		TargetBranch: input.TargetBranch, BaseCommit: input.BaseCommit,
		SourceDigest: input.SourceDigest, EvidenceDigest: input.EvidenceDigest,
		DependencyDigests: normalize(input.DependencyDigests),
		Status:            "published", CreatedAt: now.Format(time.RFC3339Nano),
		CorrelationID: input.JourneyID,
	}
	committedEvent, err := service.buildEvents(
		"CandidateIntegrated", stream, input.OperationID, now, input.JourneyID, release,
	)
	if err != nil {
		return IntegrationResult{}, err
	}
	started = append(started, committedEvent...)
	committed, err := service.append(ctx, []string{stream}, started)
	if err != nil {
		return IntegrationResult{}, err
	}
	return IntegrationResult{
		OperationID: input.OperationID, Action: "integrate",
		EventIDs: eventIDs(committed), ReleaseID: releaseID,
		Disposition: "integrated",
	}, nil
}

func normalize(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return append([]string(nil), values...)
}

func (service *IntegrationService) buildEvents(
	eventType, stream, operationID string,
	now time.Time, journeyID string, payload any,
) ([]journal.Event, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, ErrInvalidInput
	}
	seed := fmt.Sprintf("sf3\n%s\n%s\n%s\n%d", eventType, stream, operationID, 0)
	sum := sha256.Sum256([]byte(seed))
	eventID := "sf3-" + hex.EncodeToString(sum[:])[:32]
	key := fmt.Sprintf("sf3/%s/%s/%d", eventType, operationID, 0)
	return []journal.Event{{
		ID: eventID, StreamID: stream, Seq: 0, IdempotencyKey: key,
		Type: eventType, SchemaVersion: 1, EmittedAt: now,
		CorrelationID: journeyID, PayloadJSON: data,
	}}, nil
}

func (service *IntegrationService) append(
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
	committed, err := service.store.AppendBatchIfStreamHeads(ctx, expectations, events)
	if errors.Is(err, journal.ErrStreamHeadConflict) ||
		errors.Is(err, journal.ErrIdempotencyConflict) ||
		errors.Is(err, journal.ErrSequenceConflict) ||
		errors.Is(err, journal.ErrPartialEventBatchConflict) {
		return nil, fmt.Errorf("%w: integration CAS conflict", ErrStaleIntegration)
	}
	return committed, err
}

func eventIDs(events []journal.Event) []string {
	ids := make([]string, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.ID)
	}
	return ids
}
