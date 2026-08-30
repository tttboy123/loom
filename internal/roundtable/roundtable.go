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
	FactSessionCreated       = "RoundtableSessionCreated"
	FactSeatAdded            = "RoundtableSeatAdded"
	FactSeatRetired          = "RoundtableSeatRetired"
	FactSeatRejoined         = "RoundtableSeatRejoined"
	FactRoundOpened          = "RoundtableRoundOpened"
	FactMessageProposed      = "RoundtableMessageProposed"
	FactMessageRelayed       = "RoundtableMessageRelayed"
	FactMessageAck           = "RoundtableMessageAcknowledged"
	FactMessageInserted      = "RoundtableMessageInserted"
	FactMessageDropped       = "RoundtableMessageDropped"
	FactConcluded            = "RoundtableConcluded"
	FactSessionExported      = "RoundtableSessionExported"
	FactSessionImported      = "RoundtableSessionImported"
	FactSeatAttemptStarted   = "RoundtableSeatAttemptStarted"
	FactSeatAttemptSucceeded = "RoundtableSeatAttemptSucceeded"
	FactSeatAttemptFailed    = "RoundtableSeatAttemptFailed"
	FactSeatAttemptCancelled = "RoundtableSeatAttemptCancelled"
	FactRoundPauseRequested  = "RoundtableRoundPauseRequested"
	FactRoundSteered         = "RoundtableRoundSteered"
	FactSeatRetryRequested   = "RoundtableSeatRetryRequested"
	FactSeatSkipped          = "RoundtableSeatSkipped"
	FactSeatReplaced         = "RoundtableSeatReplaced"
	FactRoundDispatchFailed  = "RoundtableRoundDispatchFailed"
)

const (
	SeatAttemptRunning   = "running"
	SeatAttemptSucceeded = "succeeded"
	SeatAttemptFailed    = "failed"
	SeatAttemptCancelled = "cancelled"
)

const (
	InterventionPauseRound           = "pause_round"
	InterventionSteer                = "steer"
	InterventionRetrySeat            = "retry_seat"
	InterventionSkipSeat             = "skip_seat"
	InterventionReplaceSeat          = "replace_seat"
	InterventionCancelSeatAttempt    = "cancel_seat_attempt"
	InterventionRoundDispatchFailure = "round_dispatch_failure"
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
	MaxRoundPromptBytes  = 4096
	MaxArtifactRefs      = 16
	MaxSeats             = 16
	MinAgentSeats        = 2
	MaxAgentSeats        = 6
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
	ErrInvalidRoundtableSession       = errors.New("invalid Roundtable session")
	ErrInvalidRoundtableSeat          = errors.New("invalid Roundtable seat")
	ErrInvalidRoundtableMessage       = errors.New("invalid Roundtable message")
	ErrRoundtableSessionNotFound      = errors.New("Roundtable session not found")
	ErrRoundtableSeatNotFound         = errors.New("Roundtable seat not found")
	ErrRoundtableNotModerator         = errors.New("Roundtable seat is not the moderator")
	ErrRoundtableSeatUnavailable      = errors.New("Roundtable seat unavailable")
	ErrRoundtableAlreadyConcluded     = errors.New("Roundtable session already concluded")
	ErrRoundtableMessageNotFound      = errors.New("Roundtable message not found")
	ErrRoundtableInvalidBody          = errors.New("Roundtable message body exceeds the bounded limit")
	ErrRoundtableInvalidDigest        = errors.New("Roundtable digest is not a valid SHA-256")
	ErrRoundtableDigestMismatch       = errors.New("Roundtable message digest mismatch")
	ErrRoundtableConflict             = errors.New("Roundtable fact conflict")
	ErrRoundtableRoundNotFound        = errors.New("Roundtable round not found")
	ErrRoundtableTooManyMessages      = errors.New("Roundtable round has too many messages")
	ErrRoundtableTooManySeats         = errors.New("Roundtable session has too many seats")
	ErrRoundtableAgentSeatCount       = errors.New("Roundtable requires two to six active Agent seats")
	ErrInvalidRoundtableSeatBinding   = errors.New("invalid Roundtable frozen seat binding")
	ErrInvalidRoundtableSeatAttempt   = errors.New("invalid Roundtable seat Attempt")
	ErrRoundtableSeatAttemptConflict  = errors.New("Roundtable seat Attempt conflict")
	ErrInvalidRoundtableIntervention  = errors.New("invalid Roundtable intervention")
	ErrRoundtableInterventionConflict = errors.New("Roundtable intervention conflict")
	ErrRoundtableInterventionRequired = errors.New("Roundtable requires user intervention before conclusion")
)

// Session is the moderator-hosted ledger identity.
type Session struct {
	ID            string          `json:"id"`
	ModeratorSeat string          `json:"moderator_seat"`
	Title         string          `json:"title"`
	CreatedAt     time.Time       `json:"created_at"`
	Concluded     bool            `json:"concluded"`
	Context       *SessionContext `json:"context,omitempty"`
}

// Seat is a participant identity.
type Seat struct {
	ID          string             `json:"id"`
	DisplayName string             `json:"display_name"`
	Available   bool               `json:"available"`
	Binding     *FrozenSeatBinding `json:"binding,omitempty"`
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
	ID             string    `json:"id"`
	Sequence       int       `json:"sequence"`
	MessageCount   int       `json:"message_count"`
	Messages       []Message `json:"messages"`
	PauseRequested bool      `json:"pause_requested,omitempty"`
}

// SeatAttempt is the non-content execution projection for one Agent seat.
// Model output is referenced by digest and encrypted payload identity only.
type SeatAttempt struct {
	AttemptID              string    `json:"attempt_id"`
	RoundID                string    `json:"round_id"`
	SeatID                 string    `json:"seat_id"`
	AttemptNumber          int       `json:"attempt_number"`
	ExecutionTeamID        string    `json:"execution_team_id"`
	WorkItemID             string    `json:"work_item_id"`
	RunID                  string    `json:"run_id"`
	SegmentID              string    `json:"segment_id"`
	ClaimGeneration        int64     `json:"claim_generation"`
	RuntimeInstanceID      string    `json:"runtime_instance_id"`
	AgentInstanceID        string    `json:"agent_instance_id"`
	MembershipRevision     int       `json:"membership_revision"`
	SeatBindingDigest      string    `json:"seat_binding_digest"`
	ExecutionBindingDigest string    `json:"execution_binding_digest"`
	ContextCapsuleDigest   string    `json:"context_capsule_digest"`
	PayloadReference       string    `json:"payload_reference"`
	Status                 string    `json:"status"`
	OutputDigest           string    `json:"output_digest"`
	IncidentID             string    `json:"incident_id"`
	FailureCode            string    `json:"failure_code"`
	FailureStage           string    `json:"failure_stage"`
	Retryable              bool      `json:"retryable"`
	StartedAt              time.Time `json:"started_at"`
	CompletedAt            time.Time `json:"completed_at"`
}

// Intervention is a content-negative receipt for one governed control applied
// to an open RoundTable round. ModeratorSeat is empty only for daemon-internal
// Attempt cancellation.
type Intervention struct {
	ID                         string    `json:"id"`
	Kind                       string    `json:"kind"`
	RoundID                    string    `json:"round_id"`
	ModeratorSeat              string    `json:"moderator_seat"`
	SeatID                     string    `json:"seat_id,omitempty"`
	AttemptID                  string    `json:"attempt_id,omitempty"`
	InputID                    string    `json:"input_id,omitempty"`
	ContentDigest              string    `json:"content_digest,omitempty"`
	RequestedAttemptNumber     int       `json:"requested_attempt_number,omitempty"`
	PreviousMembershipRevision int       `json:"previous_membership_revision,omitempty"`
	MembershipRevision         int       `json:"membership_revision,omitempty"`
	PreviousBindingDigest      string    `json:"previous_binding_digest,omitempty"`
	SeatBindingDigest          string    `json:"seat_binding_digest,omitempty"`
	IncidentID                 string    `json:"incident_id,omitempty"`
	FailureCode                string    `json:"failure_code,omitempty"`
	FailureStage               string    `json:"failure_stage,omitempty"`
	Retryable                  bool      `json:"retryable,omitempty"`
	RequestedAt                time.Time `json:"requested_at"`
	Digest                     string    `json:"digest"`
}

// SeatDelivery is an ephemeral, bounded product projection of authorized
// output. It is never part of the RoundTable Journal digest.
type SeatDelivery struct {
	AttemptID            string    `json:"attempt_id"`
	SeatID               string    `json:"seat_id"`
	Status               string    `json:"status"`
	Body                 string    `json:"body"`
	AgentInputCapability string    `json:"agent_input_capability,omitempty"`
	UpdatedAt            time.Time `json:"updated_at"`
}

// View is the rebuildable Roundtable session state.
type View struct {
	Session       Session                 `json:"session"`
	Seats         map[string]Seat         `json:"seats"`
	Rounds        []Round                 `json:"rounds"`
	Messages      map[string]Message      `json:"messages"`
	Attempts      map[string]SeatAttempt  `json:"attempts,omitempty"`
	Interventions map[string]Intervention `json:"interventions,omitempty"`
	Deliveries    map[string]SeatDelivery `json:"deliveries,omitempty"`
	Digest        string                  `json:"digest"`
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
	view.Session.Context = cloneSessionContext(view.Session.Context)
	seats := make(map[string]Seat, len(view.Seats))
	for id, seat := range view.Seats {
		seat.Binding = cloneFrozenSeatBindingPointer(seat.Binding)
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
	attempts := make(map[string]SeatAttempt, len(view.Attempts))
	for id, attempt := range view.Attempts {
		attempts[id] = attempt
	}
	view.Attempts = attempts
	interventions := make(map[string]Intervention, len(view.Interventions))
	for id, intervention := range view.Interventions {
		interventions[id] = intervention
	}
	view.Interventions = interventions
	deliveries := make(map[string]SeatDelivery, len(view.Deliveries))
	for id, delivery := range view.Deliveries {
		deliveries[id] = delivery
	}
	view.Deliveries = deliveries
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
