package roundtable

import (
	"context"
	"encoding/json"
	"time"

	"loom-pi-rebuild/internal/journal"
)

type StartSeatAttemptCommand struct {
	SessionID              string
	RoundID                string
	SeatID                 string
	AttemptID              string
	AttemptNumber          int
	ExecutionTeamID        string
	WorkItemID             string
	RunID                  string
	SegmentID              string
	ClaimGeneration        int64
	RuntimeInstanceID      string
	AgentInstanceID        string
	MembershipRevision     int
	SeatBindingDigest      string
	ExecutionBindingDigest string
	ContextCapsuleDigest   string
	EmittedAt              time.Time
	CorrelationID          string
}

func (authority *Authority) StartSeatAttempt(
	ctx context.Context,
	command StartSeatAttemptCommand,
) (View, error) {
	return authority.apply(ctx, command.SessionID, func(state sessionState) (View, fact, error) {
		if !state.found {
			return View{}, fact{}, ErrRoundtableSessionNotFound
		}
		if state.session.Concluded {
			return View{}, fact{}, ErrRoundtableAlreadyConcluded
		}
		if state.session.Context == nil || roundRecordIndex(state.rounds, command.RoundID) < 0 ||
			!roundAllowsSeatAttemptStartAfterPause(
				state, command.RoundID, command.SeatID, command.AttemptNumber,
			) ||
			roundDispatchFailed(state, command.RoundID) ||
			seatSkipped(state, command.RoundID, command.SeatID) ||
			!validRoundtableID(command.AttemptID, MaxMessageIDBytes) ||
			!validRoundtableID(command.ExecutionTeamID, MaxSessionIDBytes) ||
			!validRoundtableID(command.WorkItemID, MaxSessionIDBytes) ||
			!validRoundtableID(command.RunID, MaxSessionIDBytes) ||
			!validRoundtableID(command.SegmentID, MaxSessionIDBytes) ||
			command.AttemptNumber <= 0 ||
			command.ClaimGeneration <= 0 ||
			!validRoundtableID(command.RuntimeInstanceID, MaxSessionIDBytes) ||
			!validRoundtableID(command.AgentInstanceID, MaxSessionIDBytes) ||
			!validSHA256Digest(command.SeatBindingDigest) ||
			!validSHA256Digest(command.ExecutionBindingDigest) ||
			!validSHA256Digest(command.ContextCapsuleDigest) ||
			command.MembershipRevision <= 0 || command.EmittedAt.IsZero() ||
			command.EmittedAt.Location() != time.UTC ||
			!validCorrelationID(command.CorrelationID) {
			return View{}, fact{}, ErrInvalidRoundtableSeatAttempt
		}
		seat, found := state.seats[command.SeatID]
		if !found || !seat.Available || seat.Binding == nil ||
			seat.Binding.MembershipRevision != command.MembershipRevision ||
			seat.Binding.BindingDigest != command.SeatBindingDigest ||
			seat.Binding.ExecutionBinding.BindingDigest != command.ExecutionBindingDigest ||
			seat.Binding.ExecutionBinding.RuntimeInstanceID != command.RuntimeInstanceID {
			return View{}, fact{}, ErrInvalidRoundtableSeatAttempt
		}
		if existing, found := state.attempts[command.AttemptID]; found {
			if sameSeatAttemptStart(existing, command) {
				return cloneView(state.view), fact{}, nil
			}
			return View{}, fact{}, ErrRoundtableSeatAttemptConflict
		}
		for _, existing := range state.attempts {
			if existing.RoundID == command.RoundID && existing.SeatID == command.SeatID &&
				existing.Status == SeatAttemptRunning {
				if command.AttemptNumber != existing.AttemptNumber {
					return View{}, fact{}, ErrInvalidRoundtableSeatAttempt
				}
				return View{}, fact{}, ErrRoundtableSeatAttemptConflict
			}
		}
		attemptNumber := nextSeatAttemptNumber(state, command.RoundID, command.SeatID)
		if command.AttemptNumber != attemptNumber ||
			!seatAttemptNumberAuthorized(state, command.RoundID, command.SeatID, attemptNumber) {
			return View{}, fact{}, ErrInvalidRoundtableSeatAttempt
		}
		attempt := SeatAttempt{
			AttemptID: command.AttemptID, RoundID: command.RoundID,
			SeatID: command.SeatID, AttemptNumber: attemptNumber,
			ExecutionTeamID: command.ExecutionTeamID, WorkItemID: command.WorkItemID,
			RunID: command.RunID, SegmentID: command.SegmentID,
			ClaimGeneration:        command.ClaimGeneration,
			RuntimeInstanceID:      command.RuntimeInstanceID,
			AgentInstanceID:        command.AgentInstanceID,
			MembershipRevision:     command.MembershipRevision,
			SeatBindingDigest:      command.SeatBindingDigest,
			ExecutionBindingDigest: command.ExecutionBindingDigest,
			ContextCapsuleDigest:   command.ContextCapsuleDigest,
			Status:                 SeatAttemptRunning, StartedAt: command.EmittedAt,
		}
		payload, err := json.Marshal(seatAttemptStartedPayload{
			SchemaVersion: SchemaVersion, SessionID: command.SessionID,
			Attempt: attempt,
		})
		if err != nil {
			return View{}, fact{}, err
		}
		view := cloneView(state.view)
		if view.Attempts == nil {
			view.Attempts = make(map[string]SeatAttempt)
		}
		view.Attempts[attempt.AttemptID] = attempt
		return view, fact{
			Event: journal.Event{
				ID:       authority.deterministicID("roundtable-seat-attempt-started", command.SessionID, command.AttemptID),
				StreamID: sessionStream(command.SessionID), Seq: state.head.Sequence + 1,
				IdempotencyKey: "roundtable.seat-attempt-started." + command.SessionID + "." + command.AttemptID,
				Type:           FactSeatAttemptStarted, SchemaVersion: SchemaVersion,
				EmittedAt: command.EmittedAt, CorrelationID: command.CorrelationID,
				CausationID: state.head.EventID, PayloadJSON: payload,
			},
			commandID: "seat-attempt-started:" + command.SessionID + ":" + command.AttemptID,
		}, nil
	})
}

type SucceedSeatAttemptCommand struct {
	SessionID        string
	AttemptID        string
	PayloadReference string
	OutputDigest     string
	EmittedAt        time.Time
	CorrelationID    string
}

func (authority *Authority) SucceedSeatAttempt(
	ctx context.Context,
	command SucceedSeatAttemptCommand,
) (View, error) {
	if !validRoundtableID(command.PayloadReference, MaxSessionIDBytes) ||
		!validSHA256Digest(command.OutputDigest) {
		return View{}, ErrInvalidRoundtableSeatAttempt
	}
	return authority.terminalSeatAttempt(
		ctx, command.SessionID, command.AttemptID, SeatAttemptSucceeded,
		command.PayloadReference, command.OutputDigest, "", "", "", false,
		command.EmittedAt, command.CorrelationID,
	)
}

type FailSeatAttemptCommand struct {
	SessionID     string
	AttemptID     string
	IncidentID    string
	FailureCode   string
	FailureStage  string
	Retryable     bool
	EmittedAt     time.Time
	CorrelationID string
}

func (authority *Authority) FailSeatAttempt(
	ctx context.Context,
	command FailSeatAttemptCommand,
) (View, error) {
	if !validRoundtableID(command.IncidentID, MaxSessionIDBytes) ||
		!validRoundtableID(command.FailureCode, MaxSeatIDBytes) ||
		!validRoundtableID(command.FailureStage, MaxSeatIDBytes) {
		return View{}, ErrInvalidRoundtableSeatAttempt
	}
	return authority.terminalSeatAttempt(
		ctx, command.SessionID, command.AttemptID, SeatAttemptFailed,
		"", "", command.IncidentID, command.FailureCode, command.FailureStage,
		command.Retryable, command.EmittedAt, command.CorrelationID,
	)
}

func (authority *Authority) terminalSeatAttempt(
	ctx context.Context,
	sessionID string,
	attemptID string,
	status string,
	payloadReference string,
	outputDigest string,
	incidentID string,
	failureCode string,
	failureStage string,
	retryable bool,
	emittedAt time.Time,
	correlationID string,
) (View, error) {
	if !validRoundtableID(attemptID, MaxMessageIDBytes) || emittedAt.IsZero() ||
		emittedAt.Location() != time.UTC || !validCorrelationID(correlationID) {
		return View{}, ErrInvalidRoundtableSeatAttempt
	}
	return authority.apply(ctx, sessionID, func(state sessionState) (View, fact, error) {
		attempt, found := state.attempts[attemptID]
		if !found {
			return View{}, fact{}, ErrInvalidRoundtableSeatAttempt
		}
		if attempt.Status != SeatAttemptRunning {
			if sameSeatAttemptTerminal(
				attempt, status, payloadReference, outputDigest, incidentID,
				failureCode, failureStage, retryable,
			) {
				return cloneView(state.view), fact{}, nil
			}
			return View{}, fact{}, ErrRoundtableSeatAttemptConflict
		}
		attempt.Status = status
		attempt.PayloadReference = payloadReference
		attempt.OutputDigest = outputDigest
		attempt.IncidentID = incidentID
		attempt.FailureCode = failureCode
		attempt.FailureStage = failureStage
		attempt.Retryable = retryable
		attempt.CompletedAt = emittedAt
		payload := seatAttemptTerminalPayload{
			SchemaVersion: SchemaVersion, SessionID: sessionID, Attempt: attempt,
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			return View{}, fact{}, err
		}
		factType := FactSeatAttemptSucceeded
		action := "succeeded"
		if status == SeatAttemptFailed {
			factType = FactSeatAttemptFailed
			action = "failed"
		}
		view := cloneView(state.view)
		view.Attempts[attemptID] = attempt
		return view, fact{
			Event: journal.Event{
				ID:       authority.deterministicID("roundtable-seat-attempt-"+action, sessionID, attemptID),
				StreamID: sessionStream(sessionID), Seq: state.head.Sequence + 1,
				IdempotencyKey: "roundtable.seat-attempt-" + action + "." + sessionID + "." + attemptID,
				Type:           factType, SchemaVersion: SchemaVersion, EmittedAt: emittedAt,
				CorrelationID: correlationID, CausationID: state.head.EventID,
				PayloadJSON: encoded,
			},
			commandID: "seat-attempt-" + action + ":" + sessionID + ":" + attemptID,
		}, nil
	})
}

func nextSeatAttemptNumber(state sessionState, roundID, seatID string) int {
	next := 1
	for _, attempt := range state.attempts {
		if attempt.RoundID == roundID && attempt.SeatID == seatID &&
			attempt.AttemptNumber >= next {
			next = attempt.AttemptNumber + 1
		}
	}
	return next
}

func sameSeatAttemptStart(attempt SeatAttempt, command StartSeatAttemptCommand) bool {
	return attempt.AttemptID == command.AttemptID && attempt.RoundID == command.RoundID &&
		attempt.SeatID == command.SeatID && attempt.AttemptNumber == command.AttemptNumber &&
		attempt.ExecutionTeamID == command.ExecutionTeamID &&
		attempt.WorkItemID == command.WorkItemID && attempt.RunID == command.RunID &&
		attempt.SegmentID == command.SegmentID &&
		attempt.ClaimGeneration == command.ClaimGeneration &&
		attempt.RuntimeInstanceID == command.RuntimeInstanceID &&
		attempt.AgentInstanceID == command.AgentInstanceID &&
		attempt.MembershipRevision == command.MembershipRevision &&
		attempt.SeatBindingDigest == command.SeatBindingDigest &&
		attempt.ExecutionBindingDigest == command.ExecutionBindingDigest &&
		attempt.ContextCapsuleDigest == command.ContextCapsuleDigest
}

func seatAttemptNumberAuthorized(
	state sessionState,
	roundID string,
	seatID string,
	attemptNumber int,
) bool {
	if attemptNumber == 1 {
		return true
	}
	for _, intervention := range state.interventions {
		if intervention.RoundID == roundID && intervention.SeatID == seatID &&
			intervention.RequestedAttemptNumber == attemptNumber &&
			(intervention.Kind == InterventionRetrySeat ||
				intervention.Kind == InterventionReplaceSeat) {
			return true
		}
	}
	return false
}

func roundAllowsSeatAttemptStartAfterPause(
	state sessionState,
	roundID string,
	seatID string,
	attemptNumber int,
) bool {
	if !roundPauseRequested(state, roundID) {
		return true
	}
	var pausedAt time.Time
	for _, intervention := range state.interventions {
		if intervention.Kind == InterventionPauseRound && intervention.RoundID == roundID {
			pausedAt = intervention.RequestedAt
			break
		}
	}
	if pausedAt.IsZero() {
		return false
	}
	for _, intervention := range state.interventions {
		if intervention.Kind == InterventionRetrySeat && intervention.RoundID == roundID &&
			intervention.SeatID == seatID &&
			intervention.RequestedAttemptNumber == attemptNumber &&
			intervention.RequestedAt.After(pausedAt) {
			return true
		}
	}
	return false
}

func sameSeatAttemptTerminal(
	attempt SeatAttempt,
	status, payloadReference, outputDigest, incidentID, failureCode, failureStage string,
	retryable bool,
) bool {
	return attempt.Status == status && attempt.PayloadReference == payloadReference &&
		attempt.OutputDigest == outputDigest && attempt.IncidentID == incidentID &&
		attempt.FailureCode == failureCode && attempt.FailureStage == failureStage &&
		attempt.Retryable == retryable
}

type seatAttemptStartedPayload struct {
	SchemaVersion int         `json:"schema_version"`
	SessionID     string      `json:"session_id"`
	Attempt       SeatAttempt `json:"attempt"`
}

type seatAttemptTerminalPayload struct {
	SchemaVersion int         `json:"schema_version"`
	SessionID     string      `json:"session_id"`
	Attempt       SeatAttempt `json:"attempt"`
}
