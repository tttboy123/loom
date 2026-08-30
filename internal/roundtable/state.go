package roundtable

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/journal"
)

type sessionState struct {
	found                   bool
	head                    journal.StreamHead
	session                 Session
	seats                   map[string]Seat
	seatMembershipRevisions map[string]int
	messages                map[string]Message
	attempts                map[string]SeatAttempt
	interventions           map[string]Intervention
	rounds                  []roundRecord
	view                    View
	concludedSummaryDigest  string
	concludedAt             time.Time
	importedContracts       map[string]struct{}
}

type roundRecord struct {
	id             string
	sequence       int
	messageCount   int
	messages       []string
	pauseRequested bool
}

type fact struct {
	Event     journal.Event
	commandID string
}

// ReadView rebuilds the session view from its Journal stream.
func (authority *Authority) ReadView(
	ctx context.Context,
	sessionID string,
) (View, error) {
	if authority == nil || !validRoundtableID(sessionID, MaxSessionIDBytes) {
		return View{}, ErrInvalidRoundtableSession
	}
	events, err := authority.store.ReadStream(ctx, sessionStream(sessionID))
	if err != nil {
		return View{}, err
	}
	state, err := replaySession(sessionID, events)
	if err != nil {
		return View{}, err
	}
	if !state.found {
		return View{}, ErrRoundtableSessionNotFound
	}
	return cloneView(state.view), nil
}

func (authority *Authority) apply(
	ctx context.Context,
	sessionID string,
	transition func(sessionState) (View, fact, error),
) (View, error) {
	if authority == nil || !validRoundtableID(sessionID, MaxSessionIDBytes) {
		return View{}, ErrInvalidRoundtableSession
	}
	if err := ctx.Err(); err != nil {
		return View{}, err
	}
	events, err := authority.store.ReadStream(ctx, sessionStream(sessionID))
	if err != nil {
		return View{}, err
	}
	state, err := replaySession(sessionID, events)
	if err != nil {
		return View{}, err
	}
	next, nextFact, err := transition(state)
	if err != nil {
		return View{}, err
	}
	next.Digest = digestView(next)
	if nextFact.Event.ID == "" {
		return cloneView(next), nil
	}
	if err := ctx.Err(); err != nil {
		return View{}, err
	}
	expectedSequence := int64(0)
	if state.found {
		expectedSequence = state.head.Sequence
	}
	expectations := []journal.StreamHeadExpectation{{
		StreamID: sessionStream(sessionID), Sequence: expectedSequence,
	}}
	committed, err := authority.store.AppendBatchIfStreamHeads(
		ctx, expectations, []journal.Event{nextFact.Event},
	)
	if err != nil {
		return View{}, err
	}
	if len(committed) != 1 {
		return View{}, ErrRoundtableConflict
	}
	return cloneView(next), nil
}

func (authority *Authority) messageTransition(
	ctx context.Context,
	sessionID string,
	messageID string,
	mutate func(sessionState, *Message) error,
	factType string,
	action string,
	emittedAt time.Time,
	correlationID string,
) (View, error) {
	if !validRoundtableID(messageID, MaxMessageIDBytes) ||
		emittedAt.IsZero() || emittedAt.Location() != time.UTC ||
		!validCorrelationID(correlationID) {
		return View{}, ErrInvalidRoundtableMessage
	}
	return authority.apply(ctx, sessionID, func(state sessionState) (View, fact, error) {
		if !state.found {
			return View{}, fact{}, ErrRoundtableSessionNotFound
		}
		if state.session.Concluded {
			return View{}, fact{}, ErrRoundtableAlreadyConcluded
		}
		message, ok := state.messages[messageID]
		if !ok {
			return View{}, fact{}, ErrRoundtableMessageNotFound
		}
		if err := mutate(state, &message); err != nil {
			return View{}, fact{}, err
		}
		payload, err := json.Marshal(messageTransitionPayload{
			SchemaVersion: SchemaVersion, SessionID: sessionID,
			MessageID: messageID, MessageStatus: message.Status,
			OccurredAt: emittedAt,
		})
		if err != nil {
			return View{}, fact{}, err
		}
		view := cloneView(state.view)
		view.Messages[messageID] = message
		return view, fact{
			Event: journal.Event{
				ID:       authority.deterministicID("roundtable-"+action, sessionID, messageID),
				StreamID: sessionStream(sessionID), Seq: state.head.Sequence + 1,
				IdempotencyKey: "roundtable." + action + "." + sessionID + "." + messageID,
				Type:           factType, SchemaVersion: SchemaVersion,
				EmittedAt: emittedAt, CorrelationID: correlationID,
				CausationID: state.head.EventID, PayloadJSON: payload,
			},
			commandID: action + ":" + sessionID + ":" + messageID,
		}, nil
	})
}

func replaySession(
	sessionID string,
	events []journal.Event,
) (sessionState, error) {
	state := sessionState{
		seats:                   make(map[string]Seat),
		seatMembershipRevisions: make(map[string]int),
		messages:                make(map[string]Message),
		attempts:                make(map[string]SeatAttempt),
		interventions:           make(map[string]Intervention),
		rounds:                  make([]roundRecord, 0, 4),
		importedContracts:       make(map[string]struct{}),
	}
	for index, event := range events {
		if event.StreamID != sessionStream(sessionID) ||
			event.Seq != int64(index+1) || event.SchemaVersion != SchemaVersion {
			return sessionState{}, ErrRoundtableConflict
		}
		state.head = journal.StreamHead{
			StreamID: event.StreamID, Sequence: event.Seq, EventID: event.ID,
		}
		if err := applyFact(&state, event); err != nil {
			return sessionState{}, err
		}
	}
	if !state.found {
		return state, nil
	}
	state.view = buildView(state)
	return state, nil
}

func applyFact(state *sessionState, event journal.Event) error {
	switch event.Type {
	case FactSessionCreated:
		var payload sessionCreatedPayload
		if err := decodeExact(event.PayloadJSON, &payload); err != nil ||
			payload.SessionID != strings.TrimPrefix(event.StreamID, "roundtable/session/") ||
			(payload.Context != nil && !validSessionContext(*payload.Context)) {
			return ErrRoundtableConflict
		}
		if state.found {
			return ErrRoundtableConflict
		}
		state.found = true
		state.session = Session{
			ID: payload.SessionID, ModeratorSeat: payload.ModeratorSeat,
			Title: payload.Title, CreatedAt: payload.CreatedAt,
			Context: cloneSessionContext(payload.Context),
		}
		state.seats[payload.ModeratorSeat] = Seat{
			ID: payload.ModeratorSeat, DisplayName: "Moderator", Available: true,
		}
		state.seatMembershipRevisions[payload.ModeratorSeat] = 1
		return nil
	case FactSeatRetired:
		var payload seatRetiredPayload
		if err := decodeExact(event.PayloadJSON, &payload); err != nil ||
			payload.SessionID != state.session.ID {
			return ErrRoundtableConflict
		}
		seat, ok := state.seats[payload.SeatID]
		if !ok || !seat.Available ||
			(payload.MembershipRevision != 0 &&
				payload.MembershipRevision != state.seatMembershipRevisions[payload.SeatID]) {
			return ErrRoundtableConflict
		}
		seat.Available = false
		state.seats[payload.SeatID] = seat
		return nil
	case FactSeatAdded:
		var payload seatAddedPayload
		if err := decodeExact(event.PayloadJSON, &payload); err != nil ||
			payload.SessionID != state.session.ID {
			return ErrRoundtableConflict
		}
		if err := validateReplayedSeatBinding(
			state.session, payload.SessionID, payload.SeatID, payload.Binding, 1,
		); err != nil {
			return err
		}
		if _, exists := state.seats[payload.SeatID]; exists {
			return ErrRoundtableConflict
		}
		state.seats[payload.SeatID] = Seat{
			ID: payload.SeatID, DisplayName: payload.DisplayName, Available: true,
			Binding: cloneFrozenSeatBindingPointer(payload.Binding),
		}
		state.seatMembershipRevisions[payload.SeatID] = 1
		return nil
	case FactSeatRejoined:
		var payload seatRejoinedPayload
		if err := decodeExact(event.PayloadJSON, &payload); err != nil ||
			payload.SessionID != state.session.ID || len(state.rounds) != 0 {
			return ErrRoundtableConflict
		}
		seat, exists := state.seats[payload.SeatID]
		if !exists || seat.Available ||
			payload.MembershipRevision != state.seatMembershipRevisions[payload.SeatID]+1 {
			return ErrRoundtableConflict
		}
		if err := validateReplayedSeatBinding(
			state.session, payload.SessionID, payload.SeatID, payload.Binding,
			payload.MembershipRevision,
		); err != nil {
			return err
		}
		seat.DisplayName = payload.DisplayName
		seat.Available = true
		seat.Binding = cloneFrozenSeatBindingPointer(payload.Binding)
		state.seats[payload.SeatID] = seat
		state.seatMembershipRevisions[payload.SeatID] = payload.MembershipRevision
		return nil
	case FactRoundOpened:
		var payload roundOpenedPayload
		if err := decodeExact(event.PayloadJSON, &payload); err != nil ||
			payload.SessionID != state.session.ID {
			return ErrRoundtableConflict
		}
		if roundRecordIndex(state.rounds, payload.RoundID) >= 0 {
			return ErrRoundtableConflict
		}
		state.rounds = append(state.rounds, roundRecord{
			id: payload.RoundID, sequence: len(state.rounds) + 1,
		})
		return nil
	case FactMessageProposed:
		var payload messageProposedPayload
		if err := decodeExact(event.PayloadJSON, &payload); err != nil ||
			payload.SessionID != state.session.ID ||
			digestBytes(payload.SessionID, payload.MessageID, payload.Body) != payload.BodyDigest {
			return ErrRoundtableConflict
		}
		if _, exists := state.messages[payload.MessageID]; exists {
			return ErrRoundtableConflict
		}
		state.messages[payload.MessageID] = Message{
			ID: payload.MessageID, RoundID: payload.RoundID,
			WriterSeat: payload.WriterSeat, TargetSeat: payload.TargetSeat,
			Body: payload.Body, ArtifactRefs: normalizedArtifactRefs(payload.ArtifactRefs),
			BodyDigest: payload.BodyDigest, Status: MessagePending,
			ProposedAt: payload.ProposedAt,
		}
		roundIndex := roundRecordIndex(state.rounds, payload.RoundID)
		if roundIndex < 0 {
			state.rounds = append(state.rounds, roundRecord{
				id: payload.RoundID, sequence: len(state.rounds) + 1,
			})
			roundIndex = len(state.rounds) - 1
		}
		state.rounds[roundIndex].messageCount++
		state.rounds[roundIndex].messages = append(
			state.rounds[roundIndex].messages, payload.MessageID,
		)
		return nil
	case FactMessageRelayed, FactMessageAck, FactMessageInserted, FactMessageDropped:
		var payload messageTransitionPayload
		if err := decodeExact(event.PayloadJSON, &payload); err != nil ||
			payload.SessionID != state.session.ID {
			return ErrRoundtableConflict
		}
		message, ok := state.messages[payload.MessageID]
		if !ok {
			return ErrRoundtableConflict
		}
		switch event.Type {
		case FactMessageRelayed:
			if message.Status != MessagePending {
				return ErrRoundtableConflict
			}
			message.Status = MessageRelayed
			message.RelayedAt = payload.OccurredAt
		case FactMessageAck:
			if message.Status != MessageRelayed {
				return ErrRoundtableConflict
			}
			message.Status = MessageAcknowledged
			message.AcknowledgedAt = payload.OccurredAt
		case FactMessageInserted:
			if message.Status != MessageAcknowledged {
				return ErrRoundtableConflict
			}
			message.Status = MessageInserted
		case FactMessageDropped:
			if message.Status == MessageInserted || message.Status == MessageDropped {
				return ErrRoundtableConflict
			}
			message.Status = MessageDropped
		}
		if payload.MessageStatus != message.Status {
			return ErrRoundtableConflict
		}
		state.messages[payload.MessageID] = message
		return nil
	case FactSeatAttemptStarted:
		var payload seatAttemptStartedPayload
		if err := decodeExact(event.PayloadJSON, &payload); err != nil ||
			payload.SessionID != state.session.ID ||
			!validReplayedSeatAttemptStart(*state, payload.Attempt) {
			return ErrRoundtableConflict
		}
		if _, exists := state.attempts[payload.Attempt.AttemptID]; exists {
			return ErrRoundtableConflict
		}
		state.attempts[payload.Attempt.AttemptID] = payload.Attempt
		return nil
	case FactSeatAttemptSucceeded, FactSeatAttemptFailed:
		var payload seatAttemptTerminalPayload
		if err := decodeExact(event.PayloadJSON, &payload); err != nil ||
			payload.SessionID != state.session.ID {
			return ErrRoundtableConflict
		}
		current, exists := state.attempts[payload.Attempt.AttemptID]
		if !exists || current.Status != SeatAttemptRunning ||
			!validReplayedSeatAttemptTerminal(current, payload.Attempt, event.Type) {
			return ErrRoundtableConflict
		}
		state.attempts[payload.Attempt.AttemptID] = payload.Attempt
		return nil
	case FactRoundPauseRequested, FactRoundSteered, FactSeatRetryRequested, FactSeatSkipped,
		FactRoundDispatchFailed:
		var payload interventionPayload
		if err := decodeExact(event.PayloadJSON, &payload); err != nil ||
			payload.SessionID != state.session.ID ||
			!validReplayedIntervention(*state, event, payload.Intervention) {
			return ErrRoundtableConflict
		}
		expectedKind := map[string]string{
			FactRoundPauseRequested: InterventionPauseRound,
			FactRoundSteered:        InterventionSteer,
			FactSeatRetryRequested:  InterventionRetrySeat,
			FactSeatSkipped:         InterventionSkipSeat,
			FactRoundDispatchFailed: InterventionRoundDispatchFailure,
		}[event.Type]
		if payload.Intervention.Kind != expectedKind {
			return ErrRoundtableConflict
		}
		if err := applyReplayedSimpleIntervention(state, payload.Intervention); err != nil {
			return err
		}
		state.interventions[payload.Intervention.ID] = payload.Intervention
		return nil
	case FactSeatReplaced:
		var payload seatReplacedPayload
		if err := decodeExact(event.PayloadJSON, &payload); err != nil ||
			payload.SessionID != state.session.ID ||
			payload.Intervention.Kind != InterventionReplaceSeat ||
			!validReplayedIntervention(*state, event, payload.Intervention) ||
			!validRoundtableText(payload.DisplayName, MaxDisplayNameBytes) {
			return ErrRoundtableConflict
		}
		intervention := payload.Intervention
		seat, found := state.seats[intervention.SeatID]
		if !found || !seat.Available || seat.Binding == nil ||
			activeSeatAttempt(*state, intervention.RoundID, intervention.SeatID) != nil ||
			seatSkipped(*state, intervention.RoundID, intervention.SeatID) ||
			intervention.PreviousMembershipRevision != seat.Binding.MembershipRevision ||
			intervention.MembershipRevision != seat.Binding.MembershipRevision+1 ||
			intervention.PreviousBindingDigest != seat.Binding.BindingDigest ||
			intervention.SeatBindingDigest != payload.Binding.BindingDigest ||
			intervention.RequestedAttemptNumber != nextSeatAttemptNumber(*state, intervention.RoundID, intervention.SeatID) {
			return ErrRoundtableConflict
		}
		if err := validateReplayedSeatBinding(
			state.session, state.session.ID, intervention.SeatID, &payload.Binding,
			intervention.MembershipRevision,
		); err != nil {
			return err
		}
		seat.DisplayName = payload.DisplayName
		seat.Binding = cloneFrozenSeatBindingPointer(&payload.Binding)
		state.seats[intervention.SeatID] = seat
		state.seatMembershipRevisions[intervention.SeatID] = intervention.MembershipRevision
		state.interventions[intervention.ID] = intervention
		return nil
	case FactSeatAttemptCancelled:
		var payload seatAttemptCancelledPayload
		if err := decodeExact(event.PayloadJSON, &payload); err != nil ||
			payload.SessionID != state.session.ID ||
			payload.Intervention.Kind != InterventionCancelSeatAttempt ||
			!validReplayedIntervention(*state, event, payload.Intervention) {
			return ErrRoundtableConflict
		}
		intervention := payload.Intervention
		current, found := state.attempts[intervention.AttemptID]
		if !found || current.Status != SeatAttemptRunning ||
			current.RoundID != intervention.RoundID || current.SeatID != intervention.SeatID ||
			payload.Attempt.AttemptID != intervention.AttemptID ||
			!validReplayedSeatAttemptTerminal(current, payload.Attempt, event.Type) {
			return ErrRoundtableConflict
		}
		state.attempts[intervention.AttemptID] = payload.Attempt
		state.interventions[intervention.ID] = intervention
		return nil
	case FactConcluded:
		var payload concludedPayload
		if err := decodeExact(event.PayloadJSON, &payload); err != nil ||
			payload.SessionID != state.session.ID ||
			!validSHA256Digest(payload.SummaryDigest) ||
			payload.ArtifactDigest != payload.SummaryDigest {
			return ErrRoundtableConflict
		}
		state.session.Concluded = true
		state.concludedSummaryDigest = payload.SummaryDigest
		state.concludedAt = payload.ConcludedAt
		return nil
	case FactSessionExported:
		var payload sessionExportedPayload
		if err := decodeExact(event.PayloadJSON, &payload); err != nil ||
			payload.SessionID != state.session.ID ||
			!validSHA256Digest(payload.Digest) ||
			payload.NotBefore.IsZero() || payload.ExpiresAt.IsZero() ||
			!payload.ExpiresAt.After(payload.NotBefore) {
			return ErrRoundtableConflict
		}
		return nil
	case FactSessionImported:
		var payload sessionImportedPayload
		if err := decodeExact(event.PayloadJSON, &payload); err != nil ||
			payload.SessionID != state.session.ID ||
			!validSHA256Digest(payload.ContractDigest) ||
			!validSHA256Digest(payload.ViewDigest) {
			return ErrRoundtableConflict
		}
		state.importedContracts[payload.ContractDigest] = struct{}{}
		return nil
	default:
		return ErrRoundtableConflict
	}
}

func validReplayedIntervention(
	state sessionState,
	event journal.Event,
	intervention Intervention,
) bool {
	if state.session.Concluded || state.session.Context == nil ||
		!validFrozenIntervention(intervention) ||
		event.EmittedAt != intervention.RequestedAt ||
		roundRecordIndex(state.rounds, intervention.RoundID) < 0 {
		return false
	}
	if intervention.Kind != InterventionCancelSeatAttempt &&
		(len(state.rounds) == 0 ||
			state.rounds[len(state.rounds)-1].id != intervention.RoundID) {
		return false
	}
	if _, exists := state.interventions[intervention.ID]; exists {
		return false
	}
	if intervention.Kind == InterventionCancelSeatAttempt {
		if intervention.ModeratorSeat != "" {
			return false
		}
	} else if intervention.ModeratorSeat != state.session.ModeratorSeat ||
		!state.seats[intervention.ModeratorSeat].Available {
		return false
	}
	return validInterventionShape(intervention)
}

func validInterventionShape(intervention Intervention) bool {
	noInput := intervention.InputID == "" && intervention.ContentDigest == ""
	noReplacement := intervention.PreviousMembershipRevision == 0 &&
		intervention.MembershipRevision == 0 &&
		intervention.PreviousBindingDigest == "" && intervention.SeatBindingDigest == ""
	noFailure := intervention.IncidentID == "" && intervention.FailureCode == "" &&
		intervention.FailureStage == "" && !intervention.Retryable
	switch intervention.Kind {
	case InterventionPauseRound:
		return intervention.SeatID == "" && intervention.AttemptID == "" && noInput &&
			intervention.RequestedAttemptNumber == 0 && noReplacement && noFailure
	case InterventionSteer:
		return validRoundtableID(intervention.SeatID, MaxSeatIDBytes) &&
			validRoundtableID(intervention.AttemptID, MaxMessageIDBytes) &&
			validRoundtableID(intervention.InputID, MaxSessionIDBytes) &&
			validSHA256Digest(intervention.ContentDigest) &&
			intervention.RequestedAttemptNumber == 0 && noReplacement && noFailure
	case InterventionRetrySeat:
		return validRoundtableID(intervention.SeatID, MaxSeatIDBytes) &&
			validRoundtableID(intervention.AttemptID, MaxMessageIDBytes) && noInput &&
			intervention.RequestedAttemptNumber > 1 &&
			validRetriedSeatBinding(intervention, noReplacement) && noFailure
	case InterventionSkipSeat:
		return validRoundtableID(intervention.SeatID, MaxSeatIDBytes) &&
			intervention.AttemptID == "" && noInput &&
			intervention.RequestedAttemptNumber == 0 &&
			validSkippedSeatBinding(intervention, noReplacement) && noFailure
	case InterventionReplaceSeat:
		return validRoundtableID(intervention.SeatID, MaxSeatIDBytes) &&
			intervention.AttemptID == "" && noInput &&
			intervention.RequestedAttemptNumber > 0 &&
			intervention.PreviousMembershipRevision > 0 &&
			intervention.MembershipRevision == intervention.PreviousMembershipRevision+1 &&
			validSHA256Digest(intervention.PreviousBindingDigest) &&
			validSHA256Digest(intervention.SeatBindingDigest) &&
			intervention.PreviousBindingDigest != intervention.SeatBindingDigest && noFailure
	case InterventionCancelSeatAttempt:
		return validRoundtableID(intervention.SeatID, MaxSeatIDBytes) &&
			validRoundtableID(intervention.AttemptID, MaxMessageIDBytes) && noInput &&
			intervention.RequestedAttemptNumber == 0 && noReplacement && noFailure
	case InterventionRoundDispatchFailure:
		return intervention.SeatID == "" && intervention.AttemptID == "" && noInput &&
			intervention.RequestedAttemptNumber == 0 && noReplacement &&
			validRoundtableID(intervention.IncidentID, MaxSessionIDBytes) &&
			validFailureMetadata(intervention.FailureCode) &&
			validFailureMetadata(intervention.FailureStage)
	default:
		return false
	}
}

func applyReplayedSimpleIntervention(
	state *sessionState,
	intervention Intervention,
) error {
	switch intervention.Kind {
	case InterventionPauseRound:
		for _, existing := range state.interventions {
			if existing.Kind == InterventionPauseRound && existing.RoundID == intervention.RoundID {
				return ErrRoundtableConflict
			}
		}
		state.rounds[roundRecordIndex(state.rounds, intervention.RoundID)].pauseRequested = true
		return nil
	case InterventionSteer:
		attempt, found := state.attempts[intervention.AttemptID]
		if !found || attempt.RoundID != intervention.RoundID ||
			attempt.SeatID != intervention.SeatID ||
			!steerAdmissionWithinAttempt(attempt, intervention.RequestedAt) {
			return ErrRoundtableConflict
		}
		for _, existing := range state.interventions {
			if existing.Kind == InterventionSteer && existing.InputID == intervention.InputID {
				return ErrRoundtableConflict
			}
		}
		return nil
	case InterventionRetrySeat:
		attempt, found := state.attempts[intervention.AttemptID]
		seat, seatFound := state.seats[intervention.SeatID]
		if !found || attempt.RoundID != intervention.RoundID ||
			attempt.SeatID != intervention.SeatID ||
			!seatFound || !seat.Available || seat.Binding == nil ||
			attempt.MembershipRevision != seat.Binding.MembershipRevision ||
			attempt.SeatBindingDigest != seat.Binding.BindingDigest ||
			intervention.MembershipRevision > 0 &&
				(intervention.MembershipRevision != seat.Binding.MembershipRevision ||
					intervention.SeatBindingDigest != seat.Binding.BindingDigest) ||
			intervention.RequestedAttemptNumber != attempt.AttemptNumber+1 ||
			intervention.RequestedAttemptNumber != nextSeatAttemptNumber(*state, intervention.RoundID, intervention.SeatID) ||
			(attempt.Status != SeatAttemptCancelled &&
				(attempt.Status != SeatAttemptFailed || !attempt.Retryable)) {
			return ErrRoundtableConflict
		}
		for _, existing := range state.interventions {
			if existing.Kind == InterventionRetrySeat &&
				existing.RoundID == intervention.RoundID &&
				existing.SeatID == intervention.SeatID &&
				existing.RequestedAttemptNumber == intervention.RequestedAttemptNumber {
				return ErrRoundtableConflict
			}
		}
		return nil
	case InterventionSkipSeat:
		seat, found := state.seats[intervention.SeatID]
		if !found || !seat.Available || seat.Binding == nil ||
			intervention.SeatID == state.session.ModeratorSeat ||
			activeSeatAttempt(*state, intervention.RoundID, intervention.SeatID) != nil ||
			intervention.MembershipRevision > 0 &&
				(intervention.MembershipRevision != seat.Binding.MembershipRevision ||
					intervention.SeatBindingDigest != seat.Binding.BindingDigest) {
			return ErrRoundtableConflict
		}
		for _, existing := range state.interventions {
			if existing.Kind == InterventionSkipSeat &&
				existing.RoundID == intervention.RoundID && existing.SeatID == intervention.SeatID {
				return ErrRoundtableConflict
			}
		}
		return nil
	case InterventionRoundDispatchFailure:
		if roundHasAttempts(*state, intervention.RoundID) {
			return ErrRoundtableConflict
		}
		for _, existing := range state.interventions {
			if existing.Kind == InterventionRoundDispatchFailure &&
				existing.RoundID == intervention.RoundID {
				return ErrRoundtableConflict
			}
		}
		return nil
	default:
		return ErrRoundtableConflict
	}
}

func validRetriedSeatBinding(intervention Intervention, legacyUnbound bool) bool {
	if legacyUnbound {
		return true
	}
	return intervention.PreviousMembershipRevision == 0 &&
		intervention.PreviousBindingDigest == "" &&
		intervention.MembershipRevision > 0 &&
		validSHA256Digest(intervention.SeatBindingDigest)
}

func validSkippedSeatBinding(intervention Intervention, legacyUnbound bool) bool {
	if legacyUnbound {
		return true
	}
	return intervention.PreviousMembershipRevision == 0 &&
		intervention.PreviousBindingDigest == "" &&
		intervention.MembershipRevision > 0 &&
		validSHA256Digest(intervention.SeatBindingDigest)
}

func steerAdmissionWithinAttempt(attempt SeatAttempt, admittedAt time.Time) bool {
	if admittedAt.IsZero() || admittedAt.Location() != time.UTC ||
		attempt.StartedAt.IsZero() || admittedAt.Before(attempt.StartedAt) {
		return false
	}
	switch attempt.Status {
	case SeatAttemptRunning:
		return attempt.CompletedAt.IsZero()
	case SeatAttemptSucceeded, SeatAttemptFailed, SeatAttemptCancelled:
		return !attempt.CompletedAt.IsZero() && !admittedAt.After(attempt.CompletedAt)
	default:
		return false
	}
}

func validateReplayedSeatBinding(
	session Session,
	sessionID string,
	seatID string,
	binding *FrozenSeatBinding,
	membershipRevision int,
) error {
	if session.Context == nil {
		if binding != nil {
			return ErrRoundtableConflict
		}
		return nil
	}
	if binding == nil || binding.MembershipRevision != membershipRevision {
		return ErrRoundtableConflict
	}
	if _, err := validateFrozenSeatBinding(
		sessionID, seatID, *session.Context, *binding,
	); err != nil {
		return ErrRoundtableConflict
	}
	return nil
}

func buildView(state sessionState) View {
	view := View{
		Session:       state.session,
		Seats:         make(map[string]Seat, len(state.seats)),
		Messages:      make(map[string]Message, len(state.messages)),
		Attempts:      make(map[string]SeatAttempt, len(state.attempts)),
		Interventions: make(map[string]Intervention, len(state.interventions)),
		Rounds:        make([]Round, 0, len(state.rounds)),
	}
	for id, seat := range state.seats {
		view.Seats[id] = seat
	}
	for id, message := range state.messages {
		view.Messages[id] = message
	}
	for id, attempt := range state.attempts {
		view.Attempts[id] = attempt
	}
	for id, intervention := range state.interventions {
		view.Interventions[id] = intervention
	}
	for _, record := range state.rounds {
		round := Round{
			ID: record.id, Sequence: record.sequence,
			MessageCount:   record.messageCount,
			Messages:       make([]Message, 0, len(record.messages)),
			PauseRequested: record.pauseRequested,
		}
		for _, messageID := range record.messages {
			if message, ok := state.messages[messageID]; ok {
				round.Messages = append(round.Messages, message)
			}
		}
		view.Rounds = append(view.Rounds, round)
	}
	view.Digest = digestView(view)
	return view
}

// digestView derives the canonical view digest from the full normalized
// view. Seats and messages are ordered by ID so replay and command results
// agree; rounds keep Journal order.
func digestView(view View) string {
	parts := []string{
		view.Session.ID, view.Session.ModeratorSeat, view.Session.Title,
		view.Session.CreatedAt.Format(time.RFC3339Nano), fmtBool(view.Session.Concluded),
	}
	if view.Session.Context != nil {
		parts = append(parts, "context", view.Session.Context.ConversationID,
			view.Session.Context.MissionID, view.Session.Context.TeamID,
			fmtInt(view.Session.Context.TeamVersion), view.Session.Context.WorkspaceID)
	}
	seatIDs := make([]string, 0, len(view.Seats))
	for id := range view.Seats {
		seatIDs = append(seatIDs, id)
	}
	sort.Strings(seatIDs)
	for _, id := range seatIDs {
		seat := view.Seats[id]
		parts = append(parts,
			"seat", seat.ID, seat.DisplayName, fmtBool(seat.Available))
		if seat.Binding != nil {
			parts = append(parts, "seat-binding", seat.Binding.BindingDigest)
		}
	}
	for _, round := range view.Rounds {
		parts = append(parts,
			"round", round.ID, fmtInt(round.Sequence), fmtInt(round.MessageCount))
		if round.PauseRequested {
			parts = append(parts, "pause-requested")
		}
	}
	messageIDs := make([]string, 0, len(view.Messages))
	for id := range view.Messages {
		messageIDs = append(messageIDs, id)
	}
	sort.Strings(messageIDs)
	for _, id := range messageIDs {
		message := view.Messages[id]
		parts = append(parts,
			"message", message.ID, message.RoundID, message.WriterSeat,
			message.TargetSeat, message.Status, message.BodyDigest,
			strings.Join(message.ArtifactRefs, ","),
			message.ProposedAt.Format(time.RFC3339Nano),
			message.RelayedAt.Format(time.RFC3339Nano),
			message.AcknowledgedAt.Format(time.RFC3339Nano),
		)
	}
	attemptIDs := make([]string, 0, len(view.Attempts))
	for id := range view.Attempts {
		attemptIDs = append(attemptIDs, id)
	}
	sort.Strings(attemptIDs)
	for _, id := range attemptIDs {
		attempt := view.Attempts[id]
		parts = append(parts,
			"attempt", attempt.AttemptID, attempt.RoundID, attempt.SeatID,
			fmtInt(attempt.AttemptNumber), fmtInt(attempt.MembershipRevision),
			attempt.ExecutionTeamID, attempt.WorkItemID, attempt.RunID, attempt.SegmentID,
			fmtInt64(attempt.ClaimGeneration), attempt.RuntimeInstanceID,
			attempt.AgentInstanceID, attempt.SeatBindingDigest,
			attempt.ExecutionBindingDigest, attempt.ContextCapsuleDigest,
			attempt.PayloadReference, attempt.Status, attempt.OutputDigest,
			attempt.IncidentID, attempt.FailureCode, attempt.FailureStage,
			fmtBool(attempt.Retryable),
			attempt.StartedAt.Format(time.RFC3339Nano),
			attempt.CompletedAt.Format(time.RFC3339Nano),
		)
	}
	interventionIDs := make([]string, 0, len(view.Interventions))
	for id := range view.Interventions {
		interventionIDs = append(interventionIDs, id)
	}
	sort.Strings(interventionIDs)
	for _, id := range interventionIDs {
		parts = append(parts, "intervention", view.Interventions[id].Digest)
	}
	return digestBytes(parts...)
}

func validReplayedSeatAttemptStart(state sessionState, attempt SeatAttempt) bool {
	seat, found := state.seats[attempt.SeatID]
	if !found || !seat.Available || seat.Binding == nil ||
		roundRecordIndex(state.rounds, attempt.RoundID) < 0 ||
		!roundAllowsSeatAttemptStartAfterPause(
			state, attempt.RoundID, attempt.SeatID, attempt.AttemptNumber,
		) ||
		roundDispatchFailed(state, attempt.RoundID) ||
		seatSkipped(state, attempt.RoundID, attempt.SeatID) ||
		!validRoundtableID(attempt.AttemptID, MaxMessageIDBytes) ||
		!validRoundtableID(attempt.ExecutionTeamID, MaxSessionIDBytes) ||
		!validRoundtableID(attempt.WorkItemID, MaxSessionIDBytes) ||
		!validRoundtableID(attempt.RunID, MaxSessionIDBytes) ||
		!validRoundtableID(attempt.SegmentID, MaxSessionIDBytes) || attempt.ClaimGeneration <= 0 ||
		!validRoundtableID(attempt.RuntimeInstanceID, MaxSessionIDBytes) ||
		!validRoundtableID(attempt.AgentInstanceID, MaxSessionIDBytes) ||
		attempt.AttemptNumber != nextSeatAttemptNumber(state, attempt.RoundID, attempt.SeatID) ||
		attempt.MembershipRevision != seat.Binding.MembershipRevision ||
		attempt.SeatBindingDigest != seat.Binding.BindingDigest ||
		attempt.ExecutionBindingDigest != seat.Binding.ExecutionBinding.BindingDigest ||
		attempt.RuntimeInstanceID != seat.Binding.ExecutionBinding.RuntimeInstanceID ||
		!validSHA256Digest(attempt.ContextCapsuleDigest) ||
		attempt.Status != SeatAttemptRunning || attempt.PayloadReference != "" ||
		attempt.OutputDigest != "" || attempt.IncidentID != "" ||
		attempt.FailureCode != "" || attempt.FailureStage != "" || attempt.Retryable ||
		attempt.StartedAt.IsZero() || attempt.StartedAt.Location() != time.UTC ||
		!attempt.CompletedAt.IsZero() {
		return false
	}
	for _, current := range state.attempts {
		if current.RoundID == attempt.RoundID && current.SeatID == attempt.SeatID &&
			current.Status == SeatAttemptRunning {
			return false
		}
	}
	return true
}

func validReplayedSeatAttemptTerminal(
	current SeatAttempt,
	terminal SeatAttempt,
	factType string,
) bool {
	if terminal.AttemptID != current.AttemptID || terminal.RoundID != current.RoundID ||
		terminal.SeatID != current.SeatID || terminal.AttemptNumber != current.AttemptNumber ||
		terminal.ExecutionTeamID != current.ExecutionTeamID ||
		terminal.WorkItemID != current.WorkItemID || terminal.RunID != current.RunID ||
		terminal.SegmentID != current.SegmentID ||
		terminal.ClaimGeneration != current.ClaimGeneration ||
		terminal.RuntimeInstanceID != current.RuntimeInstanceID ||
		terminal.AgentInstanceID != current.AgentInstanceID ||
		terminal.MembershipRevision != current.MembershipRevision ||
		terminal.SeatBindingDigest != current.SeatBindingDigest ||
		terminal.ExecutionBindingDigest != current.ExecutionBindingDigest ||
		terminal.ContextCapsuleDigest != current.ContextCapsuleDigest ||
		terminal.StartedAt != current.StartedAt || terminal.CompletedAt.IsZero() ||
		terminal.CompletedAt.Location() != time.UTC || terminal.CompletedAt.Before(current.StartedAt) {
		return false
	}
	if factType == FactSeatAttemptSucceeded {
		return terminal.Status == SeatAttemptSucceeded &&
			validRoundtableID(terminal.PayloadReference, MaxSessionIDBytes) &&
			validSHA256Digest(terminal.OutputDigest) && terminal.IncidentID == "" &&
			terminal.FailureCode == "" && terminal.FailureStage == "" && !terminal.Retryable
	}
	if factType == FactSeatAttemptFailed {
		return terminal.Status == SeatAttemptFailed && terminal.PayloadReference == "" &&
			terminal.OutputDigest == "" &&
			validRoundtableID(terminal.IncidentID, MaxSessionIDBytes) &&
			validRoundtableID(terminal.FailureCode, MaxSeatIDBytes) &&
			validRoundtableID(terminal.FailureStage, MaxSeatIDBytes)
	}
	return factType == FactSeatAttemptCancelled && terminal.Status == SeatAttemptCancelled &&
		terminal.PayloadReference == "" && terminal.OutputDigest == "" &&
		terminal.IncidentID == "" && terminal.FailureCode == "" &&
		terminal.FailureStage == "" && !terminal.Retryable
}

func fmtBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func fmtInt(value int) string { return strconv.Itoa(value) }

func fmtInt64(value int64) string { return strconv.FormatInt(value, 10) }

func roundRecordIndex(rounds []roundRecord, roundID string) int {
	for index := range rounds {
		if rounds[index].id == roundID {
			return index
		}
	}
	return -1
}

func sessionCreatedView(command CreateSessionCommand) View {
	return View{
		Session: Session{
			ID: command.SessionID, ModeratorSeat: command.ModeratorSeat,
			Title: command.Title, CreatedAt: command.EmittedAt,
			Context: cloneSessionContext(command.Context),
		},
		Seats: map[string]Seat{
			command.ModeratorSeat: {
				ID: command.ModeratorSeat, DisplayName: "Moderator", Available: true,
			},
		},
		Messages:      map[string]Message{},
		Attempts:      map[string]SeatAttempt{},
		Interventions: map[string]Intervention{},
		Rounds:        []Round{},
	}
}

func decodeExact(data []byte, output any) error {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		return err
	}
	return nil
}

func validRoundtableText(value string, maximum int) bool {
	return value != "" && utf8.ValidString(value) && len(value) <= maximum &&
		strings.TrimSpace(value) == value &&
		!strings.ContainsAny(value, "\x00\r\n")
}

func utf8Valid(value string) bool { return utf8.ValidString(value) }

func validCorrelationID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' ||
		value[18] != '-' || value[23] != '-' {
		return false
	}
	return true
}

// payloads

type sessionCreatedPayload struct {
	SchemaVersion int             `json:"schema_version"`
	SessionID     string          `json:"session_id"`
	ModeratorSeat string          `json:"moderator_seat"`
	Title         string          `json:"title"`
	CreatedAt     time.Time       `json:"created_at"`
	Context       *SessionContext `json:"context,omitempty"`
}

type seatRetiredPayload struct {
	SchemaVersion      int       `json:"schema_version"`
	SessionID          string    `json:"session_id"`
	SeatID             string    `json:"seat_id"`
	MembershipRevision int       `json:"membership_revision,omitempty"`
	RetiredAt          time.Time `json:"retired_at"`
}

type seatAddedPayload struct {
	SchemaVersion int                `json:"schema_version"`
	SessionID     string             `json:"session_id"`
	SeatID        string             `json:"seat_id"`
	DisplayName   string             `json:"display_name"`
	EmittedAt     time.Time          `json:"emitted_at"`
	Binding       *FrozenSeatBinding `json:"binding,omitempty"`
}

type seatRejoinedPayload struct {
	SchemaVersion      int                `json:"schema_version"`
	SessionID          string             `json:"session_id"`
	SeatID             string             `json:"seat_id"`
	DisplayName        string             `json:"display_name"`
	MembershipRevision int                `json:"membership_revision"`
	RejoinedAt         time.Time          `json:"rejoined_at"`
	Binding            *FrozenSeatBinding `json:"binding,omitempty"`
}

type roundOpenedPayload struct {
	SchemaVersion int       `json:"schema_version"`
	SessionID     string    `json:"session_id"`
	RoundID       string    `json:"round_id"`
	OpenedAt      time.Time `json:"opened_at"`
}

type messageProposedPayload struct {
	SchemaVersion int       `json:"schema_version"`
	SessionID     string    `json:"session_id"`
	RoundID       string    `json:"round_id"`
	MessageID     string    `json:"message_id"`
	WriterSeat    string    `json:"writer_seat"`
	TargetSeat    string    `json:"target_seat"`
	Body          string    `json:"body"`
	ArtifactRefs  []string  `json:"artifact_refs"`
	BodyDigest    string    `json:"body_digest"`
	ProposedAt    time.Time `json:"proposed_at"`
}

type messageTransitionPayload struct {
	SchemaVersion int       `json:"schema_version"`
	SessionID     string    `json:"session_id"`
	MessageID     string    `json:"message_id"`
	MessageStatus string    `json:"message_status"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type concludedPayload struct {
	SchemaVersion  int       `json:"schema_version"`
	SessionID      string    `json:"session_id"`
	SummaryDigest  string    `json:"summary_digest"`
	ArtifactDigest string    `json:"artifact_digest"`
	ConcludedAt    time.Time `json:"concluded_at"`
}

type sessionExportedPayload struct {
	SchemaVersion int       `json:"schema_version"`
	SessionID     string    `json:"session_id"`
	ExportID      string    `json:"export_id"`
	Digest        string    `json:"digest"`
	NotBefore     time.Time `json:"not_before"`
	ExpiresAt     time.Time `json:"expires_at"`
	ExportedAt    time.Time `json:"exported_at"`
}

type sessionImportedPayload struct {
	SchemaVersion  int       `json:"schema_version"`
	SessionID      string    `json:"session_id"`
	ImportID       string    `json:"import_id"`
	ContractDigest string    `json:"contract_digest"`
	ViewDigest     string    `json:"view_digest"`
	ImportedAt     time.Time `json:"imported_at"`
}

func (authority *Authority) deterministicID(parts ...string) string {
	return digestBytes(append([]string{"roundtable"}, parts...)...)
}
