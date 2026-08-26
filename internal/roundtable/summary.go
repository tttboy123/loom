package roundtable

import (
	"sort"
	"time"
)

// AlignmentSummary is the normalized, digest-bound artifact published at
// Conclude. It is bounded (message bodies <= 8 KiB, artifact refs are
// digest-only) and carries no credential, raw Grant or hidden reasoning.
type AlignmentSummary struct {
	SchemaVersion int            `json:"schema_version"`
	SessionID     string         `json:"session_id"`
	ModeratorSeat string         `json:"moderator_seat"`
	Title         string         `json:"title"`
	ConcludedAt   time.Time      `json:"concluded_at"`
	Seats         []SummarySeat  `json:"seats"`
	Rounds        []SummaryRound `json:"rounds"`
}

type SummarySeat struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

type SummaryRound struct {
	ID       string           `json:"id"`
	Sequence int              `json:"sequence"`
	Messages []SummaryMessage `json:"messages"`
}

type SummaryMessage struct {
	ID           string   `json:"id"`
	RoundID      string   `json:"round_id"`
	WriterSeat   string   `json:"writer_seat"`
	TargetSeat   string   `json:"target_seat"`
	Body         string   `json:"body"`
	ArtifactRefs []string `json:"artifact_refs"`
	Status       string   `json:"status"`
}

// BuildAlignmentSummary normalizes the session view into the summary that
// Conclude publishes to the Evidence Store. It is canonical: seats are
// sorted, rounds keep Journal order, and message bodies/refs are copied
// verbatim from the replayed facts. Empty artifact refs serialize as [] so
// strict JSON decoders never see null.
func BuildAlignmentSummary(view View, concludedAt time.Time) AlignmentSummary {
	seatList := view.SeatsList()
	seats := make([]SummarySeat, 0, len(seatList))
	for _, seat := range seatList {
		seats = append(seats, SummarySeat{
			ID: seat.ID, DisplayName: seat.DisplayName,
		})
	}
	rounds := make([]SummaryRound, 0, len(view.Rounds))
	for _, round := range view.Rounds {
		messages := make([]SummaryMessage, 0, len(round.Messages))
		for _, message := range round.Messages {
			refs := normalizedArtifactRefs(message.ArtifactRefs)
			sort.Strings(refs)
			messages = append(messages, SummaryMessage{
				ID: message.ID, RoundID: message.RoundID,
				WriterSeat: message.WriterSeat, TargetSeat: message.TargetSeat,
				Body: message.Body, ArtifactRefs: refs, Status: message.Status,
			})
		}
		rounds = append(rounds, SummaryRound{
			ID: round.ID, Sequence: round.Sequence, Messages: messages,
		})
	}
	return AlignmentSummary{
		SchemaVersion: SchemaVersion, SessionID: view.Session.ID,
		ModeratorSeat: view.Session.ModeratorSeat, Title: view.Session.Title,
		ConcludedAt: concludedAt,
		Seats:       seats, Rounds: rounds,
	}
}
