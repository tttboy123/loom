package integration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// CanaryInput starts the one-shot offline canary with exact bindings.
type CanaryInput struct {
	JourneyID         string `json:"journey_id"`
	OperationID       string `json:"operation_id"`
	RunID             string `json:"run_id"`
	RuntimeInstanceID string `json:"runtime_instance_id"`
	ModelID           string `json:"model_id"`
	SkillDigest       string `json:"skill_digest"`
}

type CanaryResult struct {
	OperationID string   `json:"operation_id"`
	Action      string   `json:"action"`
	EventIDs    []string `json:"event_ids"`
	CanaryID    string   `json:"canary_id,omitempty"`
	Disposition string   `json:"disposition"`
}

// StartCanary records exactly one canary run; a duplicate start is blocked
// by the idempotent CAS (no duplicate Run/Evidence/effect).
func (service *IntegrationService) StartCanary(ctx context.Context, input CanaryInput) (CanaryResult, error) {
	if input.RunID == "" || input.OperationID == "" || input.RuntimeInstanceID == "" {
		return CanaryResult{}, ErrInvalidInput
	}
	now := service.now().UTC()
	canarySum := sha256.Sum256([]byte("sf3\ncanary\n" + input.RunID))
	canaryID := "canary-" + hex.EncodeToString(canarySum[:])[:20]
	stream := "canary/" + canaryID
	existing, err := service.store.ReadStream(context.Background(), stream)
	if err != nil {
		return CanaryResult{}, err
	}
	for _, event := range existing {
		if event.Type == "CanaryStarted" || event.Type == "CanaryCompleted" {
			return CanaryResult{}, ErrDuplicateCanary
		}
	}
	canary := CanaryRun{
		CanaryID: canaryID, RunID: input.RunID,
		RuntimeInstanceID: input.RuntimeInstanceID, ModelID: input.ModelID,
		SkillDigest: input.SkillDigest, Status: "running",
		StartedAt: now.Format(time.RFC3339Nano), CorrelationID: input.JourneyID,
	}
	started, err := service.buildEvents(
		"CanaryStarted", stream, input.OperationID, now, input.JourneyID, canary,
	)
	if err != nil {
		return CanaryResult{}, err
	}
	completed := canary
	completed.Status = "completed"
	completed.EvidenceDigest = "sf3-canary-evidence"
	completed.FinishedAt = now.Add(time.Second).Format(time.RFC3339Nano)
	done, err := service.buildEvents(
		"CanaryCompleted", stream, input.OperationID, now, input.JourneyID, completed,
	)
	if err != nil {
		return CanaryResult{}, err
	}
	events := append(started, done...)
	committed, err := service.append(ctx, []string{stream}, events)
	if err != nil {
		if errors.Is(err, ErrStaleIntegration) {
			return CanaryResult{}, fmt.Errorf("%w: duplicate canary CAS", ErrDuplicateCanary)
		}
		return CanaryResult{}, err
	}
	return CanaryResult{
		OperationID: input.OperationID, Action: "start_canary",
		EventIDs: eventIDs(committed), CanaryID: canaryID,
		Disposition: "completed",
	}, nil
}

// CanaryProjection rebuilds canary state from the Journal (used by the
// read surface and the restart checkpoint).
func (service *IntegrationService) CanaryProjection(ctx context.Context) ([]CanaryRun, error) {
	events, err := service.store.ReadAll(ctx)
	if err != nil {
		return nil, err
	}
	var runs []CanaryRun
	for _, event := range events {
		if event.Type != "CanaryStarted" && event.Type != "CanaryCompleted" {
			continue
		}
		var run CanaryRun
		if err := json.Unmarshal(event.PayloadJSON, &run); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
		}
		runs = append(runs, run)
	}
	return runs, nil
}
