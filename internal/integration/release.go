package integration

import (
	"context"
	"encoding/json"
	"fmt"
)

// AdoptInput binds a later Run to the exact release revision/digest.
type AdoptInput struct {
	JourneyID   string `json:"journey_id"`
	OperationID string `json:"operation_id"`
	ReleaseID   string `json:"release_id"`
	RunID       string `json:"run_id"`
}

// AdoptByLaterRun records ReleaseAdoptedByLaterRun.
func (service *IntegrationService) AdoptByLaterRun(ctx context.Context, input AdoptInput) (IntegrationResult, error) {
	if input.ReleaseID == "" || input.RunID == "" || input.OperationID == "" {
		return IntegrationResult{}, ErrInvalidInput
	}
	now := service.now().UTC()
	stream := "release/" + input.ReleaseID
	events, err := service.buildEvents(
		"ReleaseAdoptedByLaterRun", stream, input.OperationID, now, input.JourneyID,
		map[string]any{"release_id": input.ReleaseID, "run_id": input.RunID},
	)
	if err != nil {
		return IntegrationResult{}, err
	}
	committed, err := service.append(ctx, []string{stream}, events)
	if err != nil {
		return IntegrationResult{}, err
	}
	return IntegrationResult{
		OperationID: input.OperationID, Action: "adopt",
		EventIDs: eventIDs(committed), ReleaseID: input.ReleaseID,
		Disposition: "adopted",
	}, nil
}

// RollbackInput restores the prior eligible release without rewriting
// Run/Evidence history.
type RollbackInput struct {
	JourneyID    string `json:"journey_id"`
	OperationID  string `json:"operation_id"`
	ReleaseID    string `json:"release_id"`
	RollbackToID string `json:"rollback_to_id"`
}

func (service *IntegrationService) Rollback(ctx context.Context, input RollbackInput) (IntegrationResult, error) {
	if input.ReleaseID == "" || input.RollbackToID == "" || input.OperationID == "" {
		return IntegrationResult{}, ErrInvalidInput
	}
	if input.ReleaseID == input.RollbackToID {
		return IntegrationResult{}, ErrRollbackUnavailable
	}
	now := service.now().UTC()
	stream := "release/" + input.ReleaseID
	events, err := service.buildEvents(
		"ReleaseRolledBack", stream, input.OperationID, now, input.JourneyID,
		map[string]any{"release_id": input.ReleaseID, "rollback_to_id": input.RollbackToID},
	)
	if err != nil {
		return IntegrationResult{}, err
	}
	committed, err := service.append(ctx, []string{stream}, events)
	if err != nil {
		return IntegrationResult{}, err
	}
	return IntegrationResult{
		OperationID: input.OperationID, Action: "rollback",
		EventIDs: eventIDs(committed), ReleaseID: input.RollbackToID,
		Disposition: "rolled_back",
	}, nil
}

// ReleaseProjection rebuilds release/adoption state from the Journal.
func (service *IntegrationService) ReleaseProjection(ctx context.Context) ([]ReleaseCandidate, error) {
	events, err := service.store.ReadAll(ctx)
	if err != nil {
		return nil, err
	}
	var releases []ReleaseCandidate
	for _, event := range events {
		switch event.Type {
		case "CandidateIntegrated", "ReleaseCandidatePublished":
			var release ReleaseCandidate
			if err := json.Unmarshal(event.PayloadJSON, &release); err != nil {
				return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
			}
			if release.ReleaseID == "" {
				return nil, fmt.Errorf("%w: empty release_id", ErrInvalidInput)
			}
			releases = append(releases, release)
		case "ReleaseAdoptedByLaterRun", "ReleaseRolledBack":
			var update struct {
				ReleaseID    string `json:"release_id"`
				RunID        string `json:"run_id"`
				RollbackToID string `json:"rollback_to_id"`
			}
			if err := json.Unmarshal(event.PayloadJSON, &update); err != nil {
				return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
			}
			for i := range releases {
				if releases[i].ReleaseID == update.ReleaseID {
					if update.RunID != "" {
						releases[i].AdoptedByRunID = update.RunID
					}
					if update.RollbackToID != "" {
						releases[i].Status = "rolled_back"
					}
				}
			}
		}
	}
	return releases, nil
}
