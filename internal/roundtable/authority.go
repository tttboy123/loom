package roundtable

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"time"

	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
)

// Authority is the Roundtable ledger writer: it validates each command
// against the replayed session state and appends facts through the existing
// Journal CAS. Conclude additionally publishes a digest-bound
// RoundtableAlignmentSummary Artifact to the Evidence Store and records the
// digest in the Journal.
type Authority struct {
	store    *journal.Store
	evidence *evidence.Store
	now      func() time.Time
	random   io.Reader
}

func NewAuthority(
	store *journal.Store,
	evidenceStore *evidence.Store,
	now func() time.Time,
	random io.Reader,
) (*Authority, error) {
	if store == nil || evidenceStore == nil || now == nil || random == nil {
		return nil, ErrInvalidRoundtableSession
	}
	return &Authority{
		store: store, evidence: evidenceStore, now: now, random: random,
	}, nil
}

type CommandID string

type CreateSessionCommand struct {
	SessionID     string
	ModeratorSeat string
	Title         string
	EmittedAt     time.Time
	CorrelationID string
}

func (authority *Authority) CreateSession(
	ctx context.Context,
	command CreateSessionCommand,
) (View, error) {
	return authority.apply(ctx, command.SessionID, func(state sessionState) (View, fact, error) {
		if state.found {
			return View{}, fact{}, ErrRoundtableConflict
		}
		if !validRoundtableID(command.SessionID, MaxSessionIDBytes) ||
			!validRoundtableID(command.ModeratorSeat, MaxSeatIDBytes) ||
			!validRoundtableText(command.Title, MaxSessionTitleBytes) ||
			command.EmittedAt.IsZero() || command.EmittedAt.Location() != time.UTC ||
			!validCorrelationID(command.CorrelationID) {
			return View{}, fact{}, ErrInvalidRoundtableSession
		}
		payload, err := json.Marshal(sessionCreatedPayload{
			SchemaVersion: SchemaVersion, SessionID: command.SessionID,
			ModeratorSeat: command.ModeratorSeat, Title: command.Title,
			CreatedAt: command.EmittedAt,
		})
		if err != nil {
			return View{}, fact{}, err
		}
		eventID := authority.deterministicID("roundtable-session-created", command.SessionID)
		return sessionCreatedView(command), fact{
			Event: journal.Event{
				ID: eventID, StreamID: sessionStream(command.SessionID),
				Seq: 1, IdempotencyKey: "roundtable.session-created." + command.SessionID,
				Type: FactSessionCreated, SchemaVersion: SchemaVersion,
				EmittedAt: command.EmittedAt, CorrelationID: command.CorrelationID,
				PayloadJSON: payload,
			},
			commandID: "session-created:" + command.SessionID,
		}, nil
	})
}

type AddSeatCommand struct {
	SessionID     string
	SeatID        string
	DisplayName   string
	EmittedAt     time.Time
	CorrelationID string
}

func (authority *Authority) AddSeat(
	ctx context.Context,
	command AddSeatCommand,
) (View, error) {
	return authority.apply(ctx, command.SessionID, func(state sessionState) (View, fact, error) {
		if !state.found {
			return View{}, fact{}, ErrRoundtableSessionNotFound
		}
		if state.session.Concluded {
			return View{}, fact{}, ErrRoundtableAlreadyConcluded
		}
		if !validRoundtableID(command.SeatID, MaxSeatIDBytes) ||
			!validRoundtableText(command.DisplayName, MaxDisplayNameBytes) ||
			command.EmittedAt.IsZero() || command.EmittedAt.Location() != time.UTC ||
			!validCorrelationID(command.CorrelationID) {
			return View{}, fact{}, ErrInvalidRoundtableSeat
		}
		if existing, exists := state.seats[command.SeatID]; exists {
			if existing.Available {
				if existing.DisplayName == command.DisplayName {
					return cloneView(state.view), fact{}, nil
				}
				return View{}, fact{}, ErrRoundtableConflict
			}
			if len(state.rounds) != 0 {
				return View{}, fact{}, ErrRoundtableConflict
			}
			revision := state.seatMembershipRevisions[command.SeatID] + 1
			payload, err := json.Marshal(seatRejoinedPayload{
				SchemaVersion: SchemaVersion, SessionID: command.SessionID,
				SeatID: command.SeatID, DisplayName: command.DisplayName,
				MembershipRevision: revision, RejoinedAt: command.EmittedAt,
			})
			if err != nil {
				return View{}, fact{}, err
			}
			view := cloneView(state.view)
			existing.DisplayName = command.DisplayName
			existing.Available = true
			view.Seats[command.SeatID] = existing
			revisionText := fmtInt(revision)
			return view, fact{
				Event: journal.Event{
					ID: authority.deterministicID(
						"roundtable-seat-rejoined", command.SessionID,
						command.SeatID, revisionText,
					),
					StreamID: sessionStream(command.SessionID), Seq: state.head.Sequence + 1,
					IdempotencyKey: "roundtable.seat-rejoined." + command.SessionID + "." + command.SeatID + "." + revisionText,
					Type:           FactSeatRejoined, SchemaVersion: SchemaVersion,
					EmittedAt: command.EmittedAt, CorrelationID: command.CorrelationID,
					CausationID: state.head.EventID, PayloadJSON: payload,
				},
				commandID: "seat-rejoined:" + command.SessionID + ":" + command.SeatID + ":" + revisionText,
			}, nil
		}
		if len(state.seats) >= MaxSeats {
			return View{}, fact{}, ErrRoundtableTooManySeats
		}
		payload, err := json.Marshal(seatAddedPayload{
			SchemaVersion: SchemaVersion, SessionID: command.SessionID,
			SeatID: command.SeatID, DisplayName: command.DisplayName,
			EmittedAt: command.EmittedAt,
		})
		if err != nil {
			return View{}, fact{}, err
		}
		view := cloneView(state.view)
		view.Seats[command.SeatID] = Seat{
			ID: command.SeatID, DisplayName: command.DisplayName, Available: true,
		}
		return view, fact{
			Event: journal.Event{
				ID:       authority.deterministicID("roundtable-seat-added", command.SessionID, command.SeatID),
				StreamID: sessionStream(command.SessionID), Seq: state.head.Sequence + 1,
				IdempotencyKey: "roundtable.seat-added." + command.SessionID + "." + command.SeatID,
				Type:           FactSeatAdded, SchemaVersion: SchemaVersion,
				EmittedAt: command.EmittedAt, CorrelationID: command.CorrelationID,
				CausationID: state.head.EventID, PayloadJSON: payload,
			},
			commandID: "seat-added:" + command.SessionID + ":" + command.SeatID,
		}, nil
	})
}

type RetireSeatCommand struct {
	SessionID     string
	SeatID        string
	ModeratorSeat string
	EmittedAt     time.Time
	CorrelationID string
}

func (authority *Authority) RetireSeat(
	ctx context.Context,
	command RetireSeatCommand,
) (View, error) {
	return authority.apply(ctx, command.SessionID, func(state sessionState) (View, fact, error) {
		if !state.found {
			return View{}, fact{}, ErrRoundtableSessionNotFound
		}
		if state.session.Concluded {
			return View{}, fact{}, ErrRoundtableAlreadyConcluded
		}
		if state.session.ModeratorSeat != command.ModeratorSeat ||
			!state.seats[command.ModeratorSeat].Available {
			return View{}, fact{}, ErrRoundtableNotModerator
		}
		if !validRoundtableID(command.SeatID, MaxSeatIDBytes) ||
			command.EmittedAt.IsZero() || command.EmittedAt.Location() != time.UTC ||
			!validCorrelationID(command.CorrelationID) {
			return View{}, fact{}, ErrInvalidRoundtableSeat
		}
		seat, ok := state.seats[command.SeatID]
		if !ok {
			return View{}, fact{}, ErrRoundtableSeatNotFound
		}
		if !seat.Available {
			return cloneView(state.view), fact{}, nil
		}
		revision := state.seatMembershipRevisions[command.SeatID]
		payload, err := json.Marshal(seatRetiredPayload{
			SchemaVersion: SchemaVersion, SessionID: command.SessionID,
			SeatID: command.SeatID, MembershipRevision: revision,
			RetiredAt: command.EmittedAt,
		})
		if err != nil {
			return View{}, fact{}, err
		}
		view := cloneView(state.view)
		seat.Available = false
		view.Seats[command.SeatID] = seat
		eventIDParts := []string{
			"roundtable-seat-retired", command.SessionID, command.SeatID,
		}
		idempotencyKey := "roundtable.retire." + command.SessionID + "." + command.SeatID
		if revision > 1 {
			revisionText := fmtInt(revision)
			eventIDParts = append(eventIDParts, revisionText)
			idempotencyKey += "." + revisionText
		}
		return view, fact{
			Event: journal.Event{
				ID:       authority.deterministicID(eventIDParts...),
				StreamID: sessionStream(command.SessionID), Seq: state.head.Sequence + 1,
				IdempotencyKey: idempotencyKey,
				Type:           FactSeatRetired, SchemaVersion: SchemaVersion,
				EmittedAt: command.EmittedAt, CorrelationID: command.CorrelationID,
				CausationID: state.head.EventID, PayloadJSON: payload,
			},
			commandID: "retire:" + command.SessionID + ":" + command.SeatID,
		}, nil
	})
}

type OpenRoundCommand struct {
	SessionID     string
	RoundID       string
	ModeratorSeat string
	EmittedAt     time.Time
	CorrelationID string
}

func (authority *Authority) OpenRound(
	ctx context.Context,
	command OpenRoundCommand,
) (View, error) {
	return authority.apply(ctx, command.SessionID, func(state sessionState) (View, fact, error) {
		if !state.found {
			return View{}, fact{}, ErrRoundtableSessionNotFound
		}
		if state.session.Concluded {
			return View{}, fact{}, ErrRoundtableAlreadyConcluded
		}
		if state.session.ModeratorSeat != command.ModeratorSeat ||
			!state.seats[command.ModeratorSeat].Available {
			return View{}, fact{}, ErrRoundtableNotModerator
		}
		if !validRoundtableID(command.RoundID, MaxRoundIDBytes) ||
			command.EmittedAt.IsZero() || command.EmittedAt.Location() != time.UTC ||
			!validCorrelationID(command.CorrelationID) {
			return View{}, fact{}, ErrInvalidRoundtableMessage
		}
		if len(state.rounds) >= MaxRounds {
			return View{}, fact{}, ErrRoundtableConflict
		}
		if roundRecordIndex(state.rounds, command.RoundID) >= 0 {
			return View{}, fact{}, ErrRoundtableConflict
		}
		payload, err := json.Marshal(roundOpenedPayload{
			SchemaVersion: SchemaVersion, SessionID: command.SessionID,
			RoundID: command.RoundID, OpenedAt: command.EmittedAt,
		})
		if err != nil {
			return View{}, fact{}, err
		}
		view := cloneView(state.view)
		view.Rounds = append(view.Rounds, Round{
			ID: command.RoundID, Sequence: len(view.Rounds) + 1,
		})
		return view, fact{
			Event: journal.Event{
				ID:       authority.deterministicID("roundtable-round-opened", command.SessionID, command.RoundID),
				StreamID: sessionStream(command.SessionID), Seq: state.head.Sequence + 1,
				IdempotencyKey: "roundtable.round." + command.SessionID + "." + command.RoundID,
				Type:           FactRoundOpened, SchemaVersion: SchemaVersion,
				EmittedAt: command.EmittedAt, CorrelationID: command.CorrelationID,
				CausationID: state.head.EventID, PayloadJSON: payload,
			},
			commandID: "round:" + command.SessionID + ":" + command.RoundID,
		}, nil
	})
}

type ProposeMessageCommand struct {
	SessionID     string
	RoundID       string
	MessageID     string
	WriterSeat    string
	TargetSeat    string
	Body          string
	ArtifactRefs  []string
	EmittedAt     time.Time
	CorrelationID string
}

func (authority *Authority) ProposeMessage(
	ctx context.Context,
	command ProposeMessageCommand,
) (View, error) {
	return authority.apply(ctx, command.SessionID, func(state sessionState) (View, fact, error) {
		if !state.found {
			return View{}, fact{}, ErrRoundtableSessionNotFound
		}
		if state.session.Concluded {
			return View{}, fact{}, ErrRoundtableAlreadyConcluded
		}
		if !validRoundtableID(command.RoundID, MaxRoundIDBytes) ||
			!validRoundtableID(command.MessageID, MaxMessageIDBytes) ||
			!validRoundtableID(command.WriterSeat, MaxSeatIDBytes) ||
			!validRoundtableID(command.TargetSeat, MaxSeatIDBytes) ||
			command.EmittedAt.IsZero() || command.EmittedAt.Location() != time.UTC ||
			!validCorrelationID(command.CorrelationID) {
			return View{}, fact{}, ErrInvalidRoundtableMessage
		}
		if _, ok := state.seats[command.WriterSeat]; !ok {
			return View{}, fact{}, ErrRoundtableSeatNotFound
		}
		if _, ok := state.seats[command.TargetSeat]; !ok {
			return View{}, fact{}, ErrRoundtableSeatNotFound
		}
		if !state.seats[command.WriterSeat].Available ||
			!state.seats[command.TargetSeat].Available {
			return View{}, fact{}, ErrRoundtableSeatUnavailable
		}
		if len(command.Body) == 0 || len(command.Body) > MaxMessageBodyBytes ||
			!utf8Valid(command.Body) {
			return View{}, fact{}, ErrRoundtableInvalidBody
		}
		if !validArtifactRefs(command.ArtifactRefs) {
			return View{}, fact{}, ErrInvalidRoundtableMessage
		}
		bodyDigest := digestBytes(command.SessionID, command.MessageID, command.Body)
		if existing, exists := state.messages[command.MessageID]; exists {
			if existing.BodyDigest == bodyDigest {
				return cloneView(state.view), fact{}, nil
			}
			return View{}, fact{}, ErrRoundtableConflict
		}
		if len(state.rounds) >= MaxRounds {
			return View{}, fact{}, ErrRoundtableConflict
		}
		roundIndex := roundRecordIndex(state.rounds, command.RoundID)
		if roundIndex < 0 {
			return View{}, fact{}, ErrRoundtableRoundNotFound
		}
		if state.rounds[roundIndex].messageCount >= MaxMessagesPerRound {
			return View{}, fact{}, ErrRoundtableTooManyMessages
		}
		payload, err := json.Marshal(messageProposedPayload{
			SchemaVersion: SchemaVersion, SessionID: command.SessionID,
			RoundID: command.RoundID, MessageID: command.MessageID,
			WriterSeat: command.WriterSeat, TargetSeat: command.TargetSeat,
			Body: command.Body, ArtifactRefs: normalizedArtifactRefs(command.ArtifactRefs),
			BodyDigest: bodyDigest, ProposedAt: command.EmittedAt,
		})
		if err != nil {
			return View{}, fact{}, err
		}
		view := cloneView(state.view)
		view.Messages[command.MessageID] = Message{
			ID: command.MessageID, RoundID: command.RoundID,
			WriterSeat: command.WriterSeat, TargetSeat: command.TargetSeat,
			Body: command.Body, ArtifactRefs: normalizedArtifactRefs(command.ArtifactRefs),
			BodyDigest: bodyDigest, Status: MessagePending, ProposedAt: command.EmittedAt,
		}
		roundByID(view.Rounds, command.RoundID).MessageCount++
		view.Digest = digestBytes(view.Digest, command.MessageID, bodyDigest)
		return view, fact{
			Event: journal.Event{
				ID:       authority.deterministicID("roundtable-message-proposed", command.SessionID, command.MessageID),
				StreamID: sessionStream(command.SessionID), Seq: state.head.Sequence + 1,
				IdempotencyKey: "roundtable.propose." + command.SessionID + "." + command.MessageID + "." + bodyDigest,
				Type:           FactMessageProposed, SchemaVersion: SchemaVersion,
				EmittedAt: command.EmittedAt, CorrelationID: command.CorrelationID,
				CausationID: state.head.EventID, PayloadJSON: payload,
			},
			commandID: "propose:" + command.SessionID + ":" + command.MessageID,
		}, nil
	})
}

type RelayMessageCommand struct {
	SessionID     string
	MessageID     string
	ModeratorSeat string
	EmittedAt     time.Time
	CorrelationID string
}

func (authority *Authority) RelayMessage(
	ctx context.Context,
	command RelayMessageCommand,
) (View, error) {
	return authority.messageTransition(
		ctx, command.SessionID, command.MessageID,
		func(state sessionState, message *Message) error {
			if state.session.ModeratorSeat != command.ModeratorSeat ||
				!state.seats[command.ModeratorSeat].Available {
				return ErrRoundtableNotModerator
			}
			if message.Status != MessagePending {
				return ErrRoundtableConflict
			}
			message.Status = MessageRelayed
			message.RelayedAt = command.EmittedAt
			return nil
		},
		FactMessageRelayed, "relay", command.EmittedAt, command.CorrelationID,
	)
}

type AcknowledgeMessageCommand struct {
	SessionID     string
	MessageID     string
	SeatID        string
	EmittedAt     time.Time
	CorrelationID string
}

func (authority *Authority) AcknowledgeMessage(
	ctx context.Context,
	command AcknowledgeMessageCommand,
) (View, error) {
	return authority.messageTransition(
		ctx, command.SessionID, command.MessageID,
		func(state sessionState, message *Message) error {
			seat, ok := state.seats[command.SeatID]
			if !ok || !seat.Available {
				return ErrRoundtableSeatUnavailable
			}
			if command.SeatID != message.TargetSeat {
				return ErrRoundtableSeatNotFound
			}
			if message.Status != MessageRelayed {
				return ErrRoundtableConflict
			}
			message.Status = MessageAcknowledged
			message.AcknowledgedAt = command.EmittedAt
			return nil
		},
		FactMessageAck, "ack", command.EmittedAt, command.CorrelationID,
	)
}

type InsertMessageCommand struct {
	SessionID     string
	MessageID     string
	ModeratorSeat string
	EmittedAt     time.Time
	CorrelationID string
}

func (authority *Authority) InsertMessage(
	ctx context.Context,
	command InsertMessageCommand,
) (View, error) {
	return authority.messageTransition(
		ctx, command.SessionID, command.MessageID,
		func(state sessionState, message *Message) error {
			if state.session.ModeratorSeat != command.ModeratorSeat ||
				!state.seats[command.ModeratorSeat].Available {
				return ErrRoundtableNotModerator
			}
			if message.Status != MessageAcknowledged {
				return ErrRoundtableConflict
			}
			message.Status = MessageInserted
			return nil
		},
		FactMessageInserted, "insert", command.EmittedAt, command.CorrelationID,
	)
}

type DropMessageCommand struct {
	SessionID     string
	MessageID     string
	ModeratorSeat string
	EmittedAt     time.Time
	CorrelationID string
}

func (authority *Authority) DropMessage(
	ctx context.Context,
	command DropMessageCommand,
) (View, error) {
	return authority.messageTransition(
		ctx, command.SessionID, command.MessageID,
		func(state sessionState, message *Message) error {
			if state.session.ModeratorSeat != command.ModeratorSeat ||
				!state.seats[command.ModeratorSeat].Available {
				return ErrRoundtableNotModerator
			}
			if message.Status == MessageInserted || message.Status == MessageDropped {
				return ErrRoundtableConflict
			}
			message.Status = MessageDropped
			return nil
		},
		FactMessageDropped, "drop", command.EmittedAt, command.CorrelationID,
	)
}

type ConcludeSessionCommand struct {
	SessionID     string
	ModeratorSeat string
	EmittedAt     time.Time
	CorrelationID string
}

func (authority *Authority) ConcludeSession(
	ctx context.Context,
	command ConcludeSessionCommand,
) (View, error) {
	return authority.apply(ctx, command.SessionID, func(state sessionState) (View, fact, error) {
		if !state.found {
			return View{}, fact{}, ErrRoundtableSessionNotFound
		}
		if state.session.Concluded {
			return View{}, fact{}, ErrRoundtableAlreadyConcluded
		}
		if state.session.ModeratorSeat != command.ModeratorSeat ||
			!state.seats[command.ModeratorSeat].Available {
			return View{}, fact{}, ErrRoundtableNotModerator
		}
		if command.EmittedAt.IsZero() || command.EmittedAt.Location() != time.UTC ||
			!validCorrelationID(command.CorrelationID) {
			return View{}, fact{}, ErrInvalidRoundtableSession
		}
		summary := BuildAlignmentSummary(state.view, command.EmittedAt)
		encoded, err := json.Marshal(summary)
		if err != nil {
			return View{}, fact{}, err
		}
		summaryDigest := digestBytes(string(encoded))
		// Publish-before-append is deliberate: the Journal fact must never
		// reference a missing artifact. A concurrent CAS failure can leave an
		// orphan digest-bound artifact (unreferenced, content-addressed,
		// bounded) which is harmless; the reverse ordering would risk a
		// committed fact pointing at a missing artifact.
		if _, err := authority.evidence.Publish(
			ctx, bytes.NewReader(encoded), summaryDigest,
		); err != nil {
			return View{}, fact{}, err
		}
		payload, err := json.Marshal(concludedPayload{
			SchemaVersion: SchemaVersion, SessionID: command.SessionID,
			SummaryDigest: summaryDigest, ArtifactDigest: summaryDigest,
			ConcludedAt: command.EmittedAt,
		})
		if err != nil {
			return View{}, fact{}, err
		}
		view := cloneView(state.view)
		view.Session.Concluded = true
		return view, fact{
			Event: journal.Event{
				ID:       authority.deterministicID("roundtable-concluded", command.SessionID),
				StreamID: sessionStream(command.SessionID), Seq: state.head.Sequence + 1,
				IdempotencyKey: "roundtable.conclude." + command.SessionID,
				Type:           FactConcluded, SchemaVersion: SchemaVersion,
				EmittedAt: command.EmittedAt, CorrelationID: command.CorrelationID,
				CausationID: state.head.EventID, PayloadJSON: payload,
			},
			commandID: "conclude:" + command.SessionID,
		}, nil
	})
}
