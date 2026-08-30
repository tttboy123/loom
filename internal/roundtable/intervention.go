package roundtable

import (
	"context"
	"encoding/json"
	"time"

	"loom-pi-rebuild/internal/journal"
)

const interventionDigestDomain = "loom.roundtable.intervention.v1"

type PauseRoundCommand struct {
	SessionID      string
	RoundID        string
	InterventionID string
	ModeratorSeat  string
	EmittedAt      time.Time
	CorrelationID  string
}

func (authority *Authority) PauseRound(
	ctx context.Context,
	command PauseRoundCommand,
) (View, error) {
	return authority.apply(ctx, command.SessionID, func(state sessionState) (View, fact, error) {
		if err := validateInterventionAuthority(state, command.RoundID, command.InterventionID,
			command.ModeratorSeat, command.EmittedAt, command.CorrelationID); err != nil {
			return View{}, fact{}, err
		}
		intervention := newIntervention(command.InterventionID, InterventionPauseRound,
			command.RoundID, command.ModeratorSeat, command.EmittedAt)
		if existing, found := state.interventions[command.InterventionID]; found {
			return idempotentIntervention(state, existing, intervention)
		}
		for _, existing := range state.interventions {
			if existing.Kind == InterventionPauseRound && existing.RoundID == command.RoundID {
				return View{}, fact{}, ErrRoundtableInterventionConflict
			}
		}
		view := cloneView(state.view)
		round := roundByID(view.Rounds, command.RoundID)
		round.PauseRequested = true
		view.Interventions[intervention.ID] = intervention
		return interventionTransition(state, view, FactRoundPauseRequested,
			"pause-requested", intervention, interventionPayload{
				SchemaVersion: SchemaVersion, SessionID: command.SessionID,
				Intervention: intervention,
			}, command.CorrelationID)
	})
}

type RecordSeatSteerCommand struct {
	SessionID      string
	RoundID        string
	InterventionID string
	ModeratorSeat  string
	SeatID         string
	AttemptID      string
	InputID        string
	ContentDigest  string
	EmittedAt      time.Time
	CorrelationID  string
}

func (authority *Authority) RecordSeatSteer(
	ctx context.Context,
	command RecordSeatSteerCommand,
) (View, error) {
	return authority.apply(ctx, command.SessionID, func(state sessionState) (View, fact, error) {
		if err := validateInterventionAuthority(state, command.RoundID, command.InterventionID,
			command.ModeratorSeat, command.EmittedAt, command.CorrelationID); err != nil {
			return View{}, fact{}, err
		}
		intervention := newIntervention(command.InterventionID, InterventionSteer,
			command.RoundID, command.ModeratorSeat, command.EmittedAt)
		intervention.SeatID = command.SeatID
		intervention.AttemptID = command.AttemptID
		intervention.InputID = command.InputID
		intervention.ContentDigest = command.ContentDigest
		intervention = freezeIntervention(intervention)
		if existing, found := state.interventions[command.InterventionID]; found {
			return idempotentIntervention(state, existing, intervention)
		}
		attempt, found := state.attempts[command.AttemptID]
		if !validRoundtableID(command.SeatID, MaxSeatIDBytes) ||
			!validRoundtableID(command.AttemptID, MaxMessageIDBytes) ||
			!validRoundtableID(command.InputID, MaxSessionIDBytes) ||
			!validSHA256Digest(command.ContentDigest) || !found ||
			attempt.RoundID != command.RoundID || attempt.SeatID != command.SeatID ||
			!steerAdmissionWithinAttempt(attempt, command.EmittedAt) {
			return View{}, fact{}, ErrInvalidRoundtableIntervention
		}
		for _, existing := range state.interventions {
			if existing.Kind == InterventionSteer && existing.InputID == command.InputID {
				return View{}, fact{}, ErrRoundtableInterventionConflict
			}
		}
		view := cloneView(state.view)
		view.Interventions[intervention.ID] = intervention
		return interventionTransition(state, view, FactRoundSteered, "steered",
			intervention, interventionPayload{
				SchemaVersion: SchemaVersion, SessionID: command.SessionID,
				Intervention: intervention,
			}, command.CorrelationID)
	})
}

type RecordSeatRetryCommand struct {
	SessionID                  string
	RoundID                    string
	InterventionID             string
	ModeratorSeat              string
	SeatID                     string
	AttemptID                  string
	RequestedAttemptNumber     int
	ExpectedMembershipRevision int
	ExpectedSeatBindingDigest  string
	EmittedAt                  time.Time
	CorrelationID              string
}

func (authority *Authority) RecordSeatRetry(
	ctx context.Context,
	command RecordSeatRetryCommand,
) (View, error) {
	return authority.apply(ctx, command.SessionID, func(state sessionState) (View, fact, error) {
		if err := validateInterventionAuthority(state, command.RoundID, command.InterventionID,
			command.ModeratorSeat, command.EmittedAt, command.CorrelationID); err != nil {
			return View{}, fact{}, err
		}
		if !validExpectedSeatBinding(
			command.ExpectedMembershipRevision, command.ExpectedSeatBindingDigest,
		) {
			return View{}, fact{}, ErrInvalidRoundtableIntervention
		}
		intervention := newIntervention(command.InterventionID, InterventionRetrySeat,
			command.RoundID, command.ModeratorSeat, command.EmittedAt)
		intervention.SeatID = command.SeatID
		intervention.AttemptID = command.AttemptID
		intervention.RequestedAttemptNumber = command.RequestedAttemptNumber
		if existing, found := state.interventions[command.InterventionID]; found {
			if command.ExpectedMembershipRevision > 0 &&
				(command.ExpectedMembershipRevision != existing.MembershipRevision ||
					command.ExpectedSeatBindingDigest != existing.SeatBindingDigest) {
				return View{}, fact{}, ErrRoundtableInterventionConflict
			}
			intervention.RequestedAt = existing.RequestedAt
			intervention.MembershipRevision = existing.MembershipRevision
			intervention.SeatBindingDigest = existing.SeatBindingDigest
			intervention = freezeIntervention(intervention)
			return idempotentIntervention(state, existing, intervention)
		}
		attempt, found := state.attempts[command.AttemptID]
		seat, seatFound := state.seats[command.SeatID]
		if !found || attempt.RoundID != command.RoundID || attempt.SeatID != command.SeatID ||
			!seatFound || !seat.Available || seat.Binding == nil ||
			command.RequestedAttemptNumber != nextSeatAttemptNumber(state, command.RoundID, command.SeatID) ||
			command.RequestedAttemptNumber != attempt.AttemptNumber+1 {
			return View{}, fact{}, ErrInvalidRoundtableIntervention
		}
		if attempt.MembershipRevision != seat.Binding.MembershipRevision ||
			attempt.SeatBindingDigest != seat.Binding.BindingDigest ||
			command.ExpectedMembershipRevision > 0 &&
				(command.ExpectedMembershipRevision != seat.Binding.MembershipRevision ||
					command.ExpectedSeatBindingDigest != seat.Binding.BindingDigest) {
			return View{}, fact{}, ErrRoundtableInterventionConflict
		}
		if attempt.Status != SeatAttemptCancelled &&
			(attempt.Status != SeatAttemptFailed || !attempt.Retryable) {
			return View{}, fact{}, ErrRoundtableInterventionConflict
		}
		for _, existing := range state.interventions {
			if existing.Kind == InterventionRetrySeat && existing.RoundID == command.RoundID &&
				existing.SeatID == command.SeatID &&
				existing.RequestedAttemptNumber == command.RequestedAttemptNumber {
				return View{}, fact{}, ErrRoundtableInterventionConflict
			}
		}
		intervention.MembershipRevision = seat.Binding.MembershipRevision
		intervention.SeatBindingDigest = seat.Binding.BindingDigest
		intervention = freezeIntervention(intervention)
		view := cloneView(state.view)
		view.Interventions[intervention.ID] = intervention
		return interventionTransition(state, view, FactSeatRetryRequested,
			"retry-requested", intervention, interventionPayload{
				SchemaVersion: SchemaVersion, SessionID: command.SessionID,
				Intervention: intervention,
			}, command.CorrelationID)
	})
}

type SkipSeatCommand struct {
	SessionID                  string
	RoundID                    string
	InterventionID             string
	ModeratorSeat              string
	SeatID                     string
	ExpectedMembershipRevision int
	ExpectedSeatBindingDigest  string
	EmittedAt                  time.Time
	CorrelationID              string
}

func (authority *Authority) SkipSeat(
	ctx context.Context,
	command SkipSeatCommand,
) (View, error) {
	return authority.apply(ctx, command.SessionID, func(state sessionState) (View, fact, error) {
		if err := validateInterventionAuthority(state, command.RoundID, command.InterventionID,
			command.ModeratorSeat, command.EmittedAt, command.CorrelationID); err != nil {
			return View{}, fact{}, err
		}
		if !validExpectedSeatBinding(
			command.ExpectedMembershipRevision, command.ExpectedSeatBindingDigest,
		) {
			return View{}, fact{}, ErrInvalidRoundtableIntervention
		}
		seat, found := state.seats[command.SeatID]
		if !found || !seat.Available || seat.Binding == nil ||
			command.SeatID == state.session.ModeratorSeat {
			return View{}, fact{}, ErrInvalidRoundtableIntervention
		}
		if command.ExpectedMembershipRevision > 0 &&
			(command.ExpectedMembershipRevision != seat.Binding.MembershipRevision ||
				command.ExpectedSeatBindingDigest != seat.Binding.BindingDigest) {
			return View{}, fact{}, ErrRoundtableInterventionConflict
		}
		intervention := newIntervention(command.InterventionID, InterventionSkipSeat,
			command.RoundID, command.ModeratorSeat, command.EmittedAt)
		intervention.SeatID = command.SeatID
		intervention.MembershipRevision = seat.Binding.MembershipRevision
		intervention.SeatBindingDigest = seat.Binding.BindingDigest
		intervention = freezeIntervention(intervention)
		if existing, found := state.interventions[command.InterventionID]; found {
			return idempotentIntervention(state, existing, intervention)
		}
		if activeSeatAttempt(state, command.RoundID, command.SeatID) != nil ||
			seatSkipped(state, command.RoundID, command.SeatID) {
			return View{}, fact{}, ErrRoundtableInterventionConflict
		}
		for _, existing := range state.interventions {
			if existing.Kind == InterventionSkipSeat && existing.RoundID == command.RoundID &&
				existing.SeatID == command.SeatID {
				return View{}, fact{}, ErrRoundtableInterventionConflict
			}
		}
		view := cloneView(state.view)
		view.Interventions[intervention.ID] = intervention
		return interventionTransition(state, view, FactSeatSkipped, "seat-skipped",
			intervention, interventionPayload{
				SchemaVersion: SchemaVersion, SessionID: command.SessionID,
				Intervention: intervention,
			}, command.CorrelationID)
	})
}

func validExpectedSeatBinding(revision int, digest string) bool {
	hasRevision := revision != 0
	hasDigest := digest != ""
	if !hasRevision && !hasDigest {
		return true
	}
	return revision > 0 && validSHA256Digest(digest)
}

type ReplaceSeatCommand struct {
	SessionID      string
	RoundID        string
	InterventionID string
	ModeratorSeat  string
	SeatID         string
	DisplayName    string
	Binding        FrozenSeatBinding
	EmittedAt      time.Time
	CorrelationID  string
}

func (authority *Authority) ReplaceSeat(
	ctx context.Context,
	command ReplaceSeatCommand,
) (View, error) {
	return authority.apply(ctx, command.SessionID, func(state sessionState) (View, fact, error) {
		if err := validateInterventionAuthority(state, command.RoundID, command.InterventionID,
			command.ModeratorSeat, command.EmittedAt, command.CorrelationID); err != nil {
			return View{}, fact{}, err
		}
		seat, found := state.seats[command.SeatID]
		if !found || !seat.Available || seat.Binding == nil || state.session.Context == nil ||
			command.SeatID == state.session.ModeratorSeat ||
			!validRoundtableText(command.DisplayName, MaxDisplayNameBytes) {
			return View{}, fact{}, ErrInvalidRoundtableIntervention
		}
		intervention := newIntervention(command.InterventionID, InterventionReplaceSeat,
			command.RoundID, command.ModeratorSeat, command.EmittedAt)
		intervention.SeatID = command.SeatID
		intervention.RequestedAttemptNumber = nextSeatAttemptNumber(state, command.RoundID, command.SeatID)
		intervention.PreviousMembershipRevision = seat.Binding.MembershipRevision
		intervention.MembershipRevision = command.Binding.MembershipRevision
		intervention.PreviousBindingDigest = seat.Binding.BindingDigest
		intervention.SeatBindingDigest = command.Binding.BindingDigest
		intervention = freezeIntervention(intervention)
		if existing, exists := state.interventions[command.InterventionID]; exists {
			if existing.Kind == InterventionReplaceSeat && existing.RoundID == command.RoundID &&
				existing.SeatID == command.SeatID && existing.SeatBindingDigest == command.Binding.BindingDigest &&
				seat.DisplayName == command.DisplayName && seat.Binding.BindingDigest == command.Binding.BindingDigest {
				return cloneView(state.view), fact{}, nil
			}
			return View{}, fact{}, ErrRoundtableInterventionConflict
		}
		expectedRevision := state.seatMembershipRevisions[command.SeatID] + 1
		if command.Binding.MembershipRevision != expectedRevision {
			return View{}, fact{}, ErrInvalidRoundtableSeatBinding
		}
		validated, err := validateFrozenSeatBinding(
			command.SessionID, command.SeatID, *state.session.Context, command.Binding,
		)
		if err != nil {
			return View{}, fact{}, err
		}
		if activeSeatAttempt(state, command.RoundID, command.SeatID) != nil ||
			seatSkipped(state, command.RoundID, command.SeatID) {
			return View{}, fact{}, ErrRoundtableInterventionConflict
		}
		view := cloneView(state.view)
		seat.DisplayName = command.DisplayName
		seat.Binding = cloneFrozenSeatBindingPointer(&validated)
		view.Seats[command.SeatID] = seat
		view.Interventions[intervention.ID] = intervention
		return interventionTransition(state, view, FactSeatReplaced, "seat-replaced",
			intervention, seatReplacedPayload{
				SchemaVersion: SchemaVersion, SessionID: command.SessionID,
				DisplayName: command.DisplayName, Binding: validated,
				Intervention: intervention,
			}, command.CorrelationID)
	})
}

type CancelSeatAttemptCommand struct {
	SessionID      string
	RoundID        string
	InterventionID string
	SeatID         string
	AttemptID      string
	EmittedAt      time.Time
	CorrelationID  string
}

func (authority *Authority) CancelSeatAttempt(
	ctx context.Context,
	command CancelSeatAttemptCommand,
) (View, error) {
	return authority.apply(ctx, command.SessionID, func(state sessionState) (View, fact, error) {
		interventionID := command.InterventionID
		if interventionID == "" && validRoundtableID(command.AttemptID, MaxMessageIDBytes) {
			interventionID = digestBytes(
				"roundtable-cancel-intervention", command.SessionID, command.AttemptID,
			)
		}
		if err := validateInternalIntervention(state, command.RoundID, interventionID,
			command.EmittedAt, command.CorrelationID); err != nil {
			return View{}, fact{}, err
		}
		intervention := newIntervention(interventionID, InterventionCancelSeatAttempt,
			command.RoundID, "", command.EmittedAt)
		intervention.SeatID = command.SeatID
		intervention.AttemptID = command.AttemptID
		intervention = freezeIntervention(intervention)
		if existing, found := state.interventions[interventionID]; found {
			if existing.Kind == InterventionCancelSeatAttempt &&
				existing.RoundID == command.RoundID && existing.SeatID == command.SeatID &&
				existing.AttemptID == command.AttemptID {
				return cloneView(state.view), fact{}, nil
			}
			return View{}, fact{}, ErrRoundtableInterventionConflict
		}
		for _, existing := range state.interventions {
			if existing.Kind == InterventionCancelSeatAttempt &&
				existing.RoundID == command.RoundID && existing.SeatID == command.SeatID &&
				existing.AttemptID == command.AttemptID {
				return cloneView(state.view), fact{}, nil
			}
		}
		attempt, found := state.attempts[command.AttemptID]
		if !found || attempt.RoundID != command.RoundID || attempt.SeatID != command.SeatID {
			return View{}, fact{}, ErrInvalidRoundtableIntervention
		}
		if attempt.Status != SeatAttemptRunning {
			return View{}, fact{}, ErrRoundtableInterventionConflict
		}
		attempt.Status = SeatAttemptCancelled
		attempt.CompletedAt = command.EmittedAt
		view := cloneView(state.view)
		view.Attempts[command.AttemptID] = attempt
		view.Interventions[intervention.ID] = intervention
		return interventionTransition(state, view, FactSeatAttemptCancelled,
			"seat-attempt-cancelled", intervention, seatAttemptCancelledPayload{
				SchemaVersion: SchemaVersion, SessionID: command.SessionID,
				Intervention: intervention, Attempt: attempt,
			}, command.CorrelationID)
	})
}

type RecordRoundDispatchFailureCommand struct {
	SessionID      string
	RoundID        string
	InterventionID string
	ModeratorSeat  string
	IncidentID     string
	FailureCode    string
	FailureStage   string
	Retryable      bool
	EmittedAt      time.Time
	CorrelationID  string
}

func (authority *Authority) RecordRoundDispatchFailure(
	ctx context.Context,
	command RecordRoundDispatchFailureCommand,
) (View, error) {
	return authority.apply(ctx, command.SessionID, func(state sessionState) (View, fact, error) {
		if err := validateInterventionAuthority(state, command.RoundID, command.InterventionID,
			command.ModeratorSeat, command.EmittedAt, command.CorrelationID); err != nil {
			return View{}, fact{}, err
		}
		intervention := newIntervention(command.InterventionID, InterventionRoundDispatchFailure,
			command.RoundID, command.ModeratorSeat, command.EmittedAt)
		intervention.IncidentID = command.IncidentID
		intervention.FailureCode = command.FailureCode
		intervention.FailureStage = command.FailureStage
		intervention.Retryable = command.Retryable
		intervention = freezeIntervention(intervention)
		if existing, found := state.interventions[command.InterventionID]; found {
			return idempotentIntervention(state, existing, intervention)
		}
		if !validRoundtableID(command.IncidentID, MaxSessionIDBytes) ||
			!validFailureMetadata(command.FailureCode) ||
			!validFailureMetadata(command.FailureStage) ||
			roundHasAttempts(state, command.RoundID) {
			return View{}, fact{}, ErrInvalidRoundtableIntervention
		}
		for _, existing := range state.interventions {
			if existing.Kind == InterventionRoundDispatchFailure &&
				existing.RoundID == command.RoundID {
				return View{}, fact{}, ErrRoundtableInterventionConflict
			}
		}
		view := cloneView(state.view)
		view.Interventions[intervention.ID] = intervention
		return interventionTransition(state, view, FactRoundDispatchFailed,
			"round-dispatch-failed", intervention, interventionPayload{
				SchemaVersion: SchemaVersion, SessionID: command.SessionID,
				Intervention: intervention,
			}, command.CorrelationID)
	})
}

func validateInternalIntervention(
	state sessionState,
	roundID string,
	interventionID string,
	emittedAt time.Time,
	correlationID string,
) error {
	if !state.found {
		return ErrRoundtableSessionNotFound
	}
	if state.session.Concluded {
		return ErrRoundtableAlreadyConcluded
	}
	if state.session.Context == nil || !validRoundtableID(roundID, MaxRoundIDBytes) ||
		!validRoundtableID(interventionID, MaxMessageIDBytes) ||
		emittedAt.IsZero() || emittedAt.Location() != time.UTC ||
		!validCorrelationID(correlationID) || roundRecordIndex(state.rounds, roundID) < 0 {
		return ErrInvalidRoundtableIntervention
	}
	return nil
}

func validateInterventionAuthority(
	state sessionState,
	roundID string,
	interventionID string,
	moderatorSeat string,
	emittedAt time.Time,
	correlationID string,
) error {
	if !state.found {
		return ErrRoundtableSessionNotFound
	}
	if state.session.Concluded {
		return ErrRoundtableAlreadyConcluded
	}
	if state.session.Context == nil || !validRoundtableID(roundID, MaxRoundIDBytes) ||
		!validRoundtableID(interventionID, MaxMessageIDBytes) ||
		emittedAt.IsZero() || emittedAt.Location() != time.UTC ||
		!validCorrelationID(correlationID) || len(state.rounds) == 0 ||
		state.rounds[len(state.rounds)-1].id != roundID {
		return ErrInvalidRoundtableIntervention
	}
	if state.session.ModeratorSeat != moderatorSeat ||
		!state.seats[moderatorSeat].Available {
		return ErrRoundtableNotModerator
	}
	return nil
}

func newIntervention(
	id string,
	kind string,
	roundID string,
	moderatorSeat string,
	requestedAt time.Time,
) Intervention {
	return freezeIntervention(Intervention{
		ID: id, Kind: kind, RoundID: roundID, ModeratorSeat: moderatorSeat,
		RequestedAt: requestedAt,
	})
}

func freezeIntervention(input Intervention) Intervention {
	input.Digest = ""
	encoded, _ := json.Marshal(struct {
		Domain       string       `json:"domain"`
		Intervention Intervention `json:"intervention"`
	}{Domain: interventionDigestDomain, Intervention: input})
	input.Digest = digestBytes(string(encoded))
	return input
}

func validFrozenIntervention(input Intervention) bool {
	if !validRoundtableID(input.ID, MaxMessageIDBytes) ||
		!validRoundtableID(input.RoundID, MaxRoundIDBytes) ||
		input.RequestedAt.IsZero() || input.RequestedAt.Location() != time.UTC ||
		!validSHA256Digest(input.Digest) {
		return false
	}
	if input.Kind == InterventionCancelSeatAttempt {
		if input.ModeratorSeat != "" {
			return false
		}
	} else if !validRoundtableID(input.ModeratorSeat, MaxSeatIDBytes) {
		return false
	}
	expected := freezeIntervention(input)
	return expected == input
}

func idempotentIntervention(
	state sessionState,
	existing Intervention,
	expected Intervention,
) (View, fact, error) {
	if existing == expected {
		return cloneView(state.view), fact{}, nil
	}
	return View{}, fact{}, ErrRoundtableInterventionConflict
}

func interventionTransition(
	state sessionState,
	view View,
	factType string,
	action string,
	intervention Intervention,
	payload any,
	correlationID string,
) (View, fact, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return View{}, fact{}, err
	}
	targetID := intervention.InterventionTargetID()
	return view, fact{
		Event: journal.Event{
			ID:       deterministicInterventionEventID(action, state.session.ID, intervention.ID, targetID),
			StreamID: sessionStream(state.session.ID), Seq: state.head.Sequence + 1,
			IdempotencyKey: "roundtable." + action + "." + state.session.ID + "." + intervention.ID + "." + targetID,
			Type:           factType, SchemaVersion: SchemaVersion, EmittedAt: intervention.RequestedAt,
			CorrelationID: correlationID,
			CausationID:   state.head.EventID, PayloadJSON: encoded,
		},
		commandID: action + ":" + state.session.ID + ":" + intervention.ID,
	}, nil
}

func deterministicInterventionEventID(action, sessionID, interventionID, targetID string) string {
	return digestBytes("roundtable", "roundtable-"+action, sessionID, interventionID, targetID)
}

func (intervention Intervention) InterventionTargetID() string {
	if intervention.AttemptID != "" {
		return intervention.AttemptID
	}
	if intervention.SeatID != "" {
		return intervention.SeatID
	}
	return intervention.RoundID
}

func activeSeatAttempt(state sessionState, roundID, seatID string) *SeatAttempt {
	for _, attempt := range state.attempts {
		if attempt.RoundID == roundID && attempt.SeatID == seatID &&
			attempt.Status == SeatAttemptRunning {
			clone := attempt
			return &clone
		}
	}
	return nil
}

func roundHasAttempts(state sessionState, roundID string) bool {
	for _, attempt := range state.attempts {
		if attempt.RoundID == roundID {
			return true
		}
	}
	return false
}

func roundDispatchFailed(state sessionState, roundID string) bool {
	for _, intervention := range state.interventions {
		if intervention.Kind == InterventionRoundDispatchFailure &&
			intervention.RoundID == roundID {
			return true
		}
	}
	return false
}

func roundPauseRequested(state sessionState, roundID string) bool {
	index := roundRecordIndex(state.rounds, roundID)
	return index >= 0 && state.rounds[index].pauseRequested
}

func seatSkipped(state sessionState, roundID, seatID string) bool {
	for _, intervention := range state.interventions {
		if intervention.Kind == InterventionSkipSeat && intervention.RoundID == roundID &&
			intervention.SeatID == seatID {
			return true
		}
	}
	return false
}

func validFailureMetadata(value string) bool {
	if !validRoundtableID(value, MaxSeatIDBytes) {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') &&
			(character < '0' || character > '9') &&
			character != '_' && character != '-' && character != '.' {
			return false
		}
	}
	return true
}

type interventionPayload struct {
	SchemaVersion int          `json:"schema_version"`
	SessionID     string       `json:"session_id"`
	Intervention  Intervention `json:"intervention"`
}

type seatReplacedPayload struct {
	SchemaVersion int               `json:"schema_version"`
	SessionID     string            `json:"session_id"`
	DisplayName   string            `json:"display_name"`
	Binding       FrozenSeatBinding `json:"binding"`
	Intervention  Intervention      `json:"intervention"`
}

type seatAttemptCancelledPayload struct {
	SchemaVersion int          `json:"schema_version"`
	SessionID     string       `json:"session_id"`
	Intervention  Intervention `json:"intervention"`
	Attempt       SeatAttempt  `json:"attempt"`
}
