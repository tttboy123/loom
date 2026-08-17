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
	found                  bool
	head                   journal.StreamHead
	session                Session
	seats                  map[string]Seat
	messages               map[string]Message
	rounds                 []roundRecord
	view                   View
	concludedSummaryDigest string
	concludedAt            time.Time
	importedContracts      map[string]struct{}
}

type roundRecord struct {
	id           string
	sequence     int
	messageCount int
	messages     []string
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
		seats:             make(map[string]Seat),
		messages:          make(map[string]Message),
		rounds:            make([]roundRecord, 0, 4),
		importedContracts: make(map[string]struct{}),
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
			payload.SessionID != strings.TrimPrefix(event.StreamID, "roundtable/session/") {
			return ErrRoundtableConflict
		}
		if state.found {
			return ErrRoundtableConflict
		}
		state.found = true
		state.session = Session{
			ID: payload.SessionID, ModeratorSeat: payload.ModeratorSeat,
			Title: payload.Title, CreatedAt: payload.CreatedAt,
		}
		state.seats[payload.ModeratorSeat] = Seat{
			ID: payload.ModeratorSeat, DisplayName: "Moderator", Available: true,
		}
		return nil
	case FactSeatRetired:
		var payload seatRetiredPayload
		if err := decodeExact(event.PayloadJSON, &payload); err != nil ||
			payload.SessionID != state.session.ID {
			return ErrRoundtableConflict
		}
		seat, ok := state.seats[payload.SeatID]
		if !ok || !seat.Available {
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
		if _, exists := state.seats[payload.SeatID]; exists {
			return ErrRoundtableConflict
		}
		state.seats[payload.SeatID] = Seat{
			ID: payload.SeatID, DisplayName: payload.DisplayName, Available: true,
		}
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

func buildView(state sessionState) View {
	view := View{
		Session:  state.session,
		Seats:    make(map[string]Seat, len(state.seats)),
		Messages: make(map[string]Message, len(state.messages)),
		Rounds:   make([]Round, 0, len(state.rounds)),
	}
	for id, seat := range state.seats {
		view.Seats[id] = seat
	}
	for id, message := range state.messages {
		view.Messages[id] = message
	}
	for _, record := range state.rounds {
		round := Round{
			ID: record.id, Sequence: record.sequence,
			MessageCount: record.messageCount,
			Messages:     make([]Message, 0, len(record.messages)),
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
	seatIDs := make([]string, 0, len(view.Seats))
	for id := range view.Seats {
		seatIDs = append(seatIDs, id)
	}
	sort.Strings(seatIDs)
	for _, id := range seatIDs {
		seat := view.Seats[id]
		parts = append(parts,
			"seat", seat.ID, seat.DisplayName, fmtBool(seat.Available))
	}
	for _, round := range view.Rounds {
		parts = append(parts,
			"round", round.ID, fmtInt(round.Sequence), fmtInt(round.MessageCount))
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
	return digestBytes(parts...)
}

func fmtBool(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func fmtInt(value int) string { return strconv.Itoa(value) }

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
		},
		Seats: map[string]Seat{
			command.ModeratorSeat: {
				ID: command.ModeratorSeat, DisplayName: "Moderator", Available: true,
			},
		},
		Messages: map[string]Message{},
		Rounds:   []Round{},
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
	SchemaVersion int       `json:"schema_version"`
	SessionID     string    `json:"session_id"`
	ModeratorSeat string    `json:"moderator_seat"`
	Title         string    `json:"title"`
	CreatedAt     time.Time `json:"created_at"`
}

type seatRetiredPayload struct {
	SchemaVersion int       `json:"schema_version"`
	SessionID     string    `json:"session_id"`
	SeatID        string    `json:"seat_id"`
	RetiredAt     time.Time `json:"retired_at"`
}

type seatAddedPayload struct {
	SchemaVersion int       `json:"schema_version"`
	SessionID     string    `json:"session_id"`
	SeatID        string    `json:"seat_id"`
	DisplayName   string    `json:"display_name"`
	EmittedAt     time.Time `json:"emitted_at"`
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
