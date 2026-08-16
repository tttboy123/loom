// Package roundtable implements the Loom Governed Handoff ledger: a moderator
// hosts multiple independent seats and relays bounded, digest-bound messages
// with per-hop confirmation. It reuses the existing Journal CAS and Evidence
// Store; it does not create a second Journal, Artifact, ContextPacket,
// decision or projection authority.
package roundtable

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Fact types written to the Roundtable Journal stream.
const (
	FactSessionCreated  = "RoundtableSessionCreated"
	FactSeatAdded       = "RoundtableSeatAdded"
	FactSeatRetired     = "RoundtableSeatRetired"
	FactRoundOpened     = "RoundtableRoundOpened"
	FactMessageProposed = "RoundtableMessageProposed"
	FactMessageRelayed  = "RoundtableMessageRelayed"
	FactMessageAck      = "RoundtableMessageAcknowledged"
	FactMessageInserted = "RoundtableMessageInserted"
	FactMessageDropped  = "RoundtableMessageDropped"
	FactConcluded       = "RoundtableConcluded"
)

// Message status lifecycle.
const (
	MessagePending      = "pending"
	MessageRelayed      = "relayed"
	MessageAcknowledged = "acknowledged"
	MessageInserted     = "inserted"
	MessageDropped      = "dropped"
)

// Bounds.
const (
	MaxMessageBodyBytes  = 8 << 10 // 8 KiB per the product brief
	MaxArtifactRefs      = 16
	MaxSeats             = 16
	MaxRounds            = 256
	MaxMessagesPerRound  = 1024
	MaxSessionTitleBytes = 256
	MaxDisplayNameBytes  = 128
	MaxSessionIDBytes    = 128
	MaxSeatIDBytes       = 128
	MaxMessageIDBytes    = 128
	MaxRoundIDBytes      = 128
	SchemaVersion        = 1
)

// Typed errors.
var (
	ErrInvalidRoundtableSession   = errors.New("invalid Roundtable session")
	ErrInvalidRoundtableSeat      = errors.New("invalid Roundtable seat")
	ErrInvalidRoundtableMessage   = errors.New("invalid Roundtable message")
	ErrRoundtableSessionNotFound  = errors.New("Roundtable session not found")
	ErrRoundtableSeatNotFound     = errors.New("Roundtable seat not found")
	ErrRoundtableNotModerator     = errors.New("Roundtable seat is not the moderator")
	ErrRoundtableSeatUnavailable  = errors.New("Roundtable seat unavailable")
	ErrRoundtableAlreadyConcluded = errors.New("Roundtable session already concluded")
	ErrRoundtableMessageNotFound  = errors.New("Roundtable message not found")
	ErrRoundtableInvalidBody      = errors.New("Roundtable message body exceeds the bounded limit")
	ErrRoundtableInvalidDigest    = errors.New("Roundtable digest is not a valid SHA-256")
	ErrRoundtableDigestMismatch   = errors.New("Roundtable message digest mismatch")
	ErrRoundtableConflict         = errors.New("Roundtable fact conflict")
	ErrRoundtableRoundNotFound    = errors.New("Roundtable round not found")
	ErrRoundtableTooManyMessages  = errors.New("Roundtable round has too many messages")
	ErrRoundtableTooManySeats     = errors.New("Roundtable session has too many seats")
)

// Session is the moderator-hosted ledger identity.
type Session struct {
	ID            string    `json:"id"`
	ModeratorSeat string    `json:"moderator_seat"`
	Title         string    `json:"title"`
	CreatedAt     time.Time `json:"created_at"`
	Concluded     bool      `json:"concluded"`
}

// Seat is a participant identity.
type Seat struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	Available   bool   `json:"available"`
}

// Message is a bounded, digest-bound relayed unit.
type Message struct {
	ID             string    `json:"id"`
	RoundID        string    `json:"round_id"`
	WriterSeat     string    `json:"writer_seat"`
	TargetSeat     string    `json:"target_seat"`
	Body           string    `json:"body"`
	ArtifactRefs   []string  `json:"artifact_refs"`
	BodyDigest     string    `json:"body_digest"`
	Status         string    `json:"status"`
	ProposedAt     time.Time `json:"proposed_at"`
	RelayedAt      time.Time `json:"relayed_at"`
	AcknowledgedAt time.Time `json:"acknowledged_at"`
}

// Round groups messages under one exchange.
type Round struct {
	ID           string    `json:"id"`
	Sequence     int       `json:"sequence"`
	MessageCount int       `json:"message_count"`
	Messages     []Message `json:"messages"`
}

// View is the rebuildable Roundtable session state.
type View struct {
	Session  Session            `json:"session"`
	Seats    map[string]Seat    `json:"seats"`
	Rounds   []Round            `json:"rounds"`
	Messages map[string]Message `json:"messages"`
	Digest   string             `json:"digest"`
}

// RoundsMessages returns the ordered messages of a round.
func (view *View) RoundsMessages(roundID string) []Message {
	if view == nil {
		return nil
	}
	round := roundByID(view.Rounds, roundID)
	if round == nil {
		return nil
	}
	return append([]Message(nil), round.Messages...)
}

// SeatsList returns the seats sorted by ID.
func (view *View) SeatsList() []Seat {
	if view == nil {
		return nil
	}
	out := make([]Seat, 0, len(view.Seats))
	for _, seat := range view.Seats {
		out = append(out, seat)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func roundByID(rounds []Round, roundID string) *Round {
	for index := range rounds {
		if rounds[index].ID == roundID {
			return &rounds[index]
		}
	}
	return nil
}

func cloneView(view View) View {
	seats := make(map[string]Seat, len(view.Seats))
	for id, seat := range view.Seats {
		seats[id] = seat
	}
	messages := make(map[string]Message, len(view.Messages))
	for id, message := range view.Messages {
		messages[id] = message
	}
	rounds := make([]Round, len(view.Rounds))
	for index, round := range view.Rounds {
		rounds[index] = round
		rounds[index].Messages = append([]Message{}, round.Messages...)
	}
	view.Seats = seats
	view.Messages = messages
	view.Rounds = rounds
	return view
}

func sessionStream(sessionID string) string {
	return "roundtable/session/" + sessionID
}

func validRoundtableID(value string, maximum int) bool {
	if value == "" || len(value) > maximum || !utf8.ValidString(value) ||
		strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range value {
		if character == '\x00' || character == '\n' || character == '\r' {
			return false
		}
	}
	return true
}

func validSHA256Digest(value string) bool {
	if len(value) != 64 {
		return false
	}
	if _, err := hex.DecodeString(value); err != nil {
		return false
	}
	return strings.ToLower(value) == value
}

func validArtifactRefs(refs []string) bool {
	if len(refs) > MaxArtifactRefs {
		return false
	}
	seen := make(map[string]struct{}, len(refs))
	for _, ref := range refs {
		if !validSHA256Digest(ref) {
			return false
		}
		if _, exists := seen[ref]; exists {
			return false
		}
		seen[ref] = struct{}{}
	}
	return true
}

// normalizedArtifactRefs returns a non-nil copy of refs so the JSON wire
// form is always an empty array rather than null (Swift decoders reject
// null for non-optional collections).
func normalizedArtifactRefs(refs []string) []string {
	if refs == nil {
		return []string{}
	}
	return append([]string{}, refs...)
}

func digestBytes(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}
