package roundtable

import (
	"context"
	"sort"
	"strings"
)

const sessionStreamPrefix = "roundtable/session/"

// ReconcileInterruptedAttempts terminalizes Attempts that were still running
// when the previous daemon stopped. It never resumes Provider work.
func (authority *Authority) ReconcileInterruptedAttempts(
	ctx context.Context,
	correlationID string,
) (int, error) {
	if authority == nil || authority.store == nil || ctx == nil || ctx.Err() != nil ||
		!validCorrelationID(correlationID) {
		return 0, ErrInvalidRoundtableSeatAttempt
	}
	events, err := authority.store.ReadAll(ctx)
	if err != nil {
		return 0, err
	}
	sessionSet := make(map[string]struct{})
	for _, event := range events {
		if strings.HasPrefix(event.StreamID, sessionStreamPrefix) {
			sessionID := strings.TrimPrefix(event.StreamID, sessionStreamPrefix)
			if validRoundtableID(sessionID, MaxSessionIDBytes) {
				sessionSet[sessionID] = struct{}{}
			}
		}
	}
	sessionIDs := make([]string, 0, len(sessionSet))
	for sessionID := range sessionSet {
		sessionIDs = append(sessionIDs, sessionID)
	}
	sort.Strings(sessionIDs)
	reconciled := 0
	for _, sessionID := range sessionIDs {
		view, readErr := authority.ReadView(ctx, sessionID)
		if readErr != nil {
			return reconciled, readErr
		}
		attemptIDs := make([]string, 0, len(view.Attempts))
		for attemptID, attempt := range view.Attempts {
			if attempt.Status == SeatAttemptRunning {
				attemptIDs = append(attemptIDs, attemptID)
			}
		}
		sort.Strings(attemptIDs)
		for _, attemptID := range attemptIDs {
			attempt := view.Attempts[attemptID]
			if _, cancelErr := authority.CancelSeatAttempt(ctx, CancelSeatAttemptCommand{
				SessionID: sessionID, RoundID: attempt.RoundID,
				SeatID: attempt.SeatID, AttemptID: attempt.AttemptID,
				EmittedAt: authority.now().UTC(), CorrelationID: correlationID,
			}); cancelErr != nil {
				return reconciled, cancelErr
			}
			reconciled++
		}
	}
	return reconciled, nil
}
