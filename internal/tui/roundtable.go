package tui

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/roundtable"
)

// RoundtableClient is the governed-handoff ledger surface used by the TUI.
// Every write is validated by the daemon against the replayed Journal stream
// and appended through the existing Journal CAS; Conclude publishes a
// digest-bound AlignmentSummary Artifact.
type RoundtableClient interface {
	RoundtableCreateSession(context.Context, RoundtableSessionCreateRequest) (roundtable.View, error)
	RoundtableAddSeat(context.Context, RoundtableAddSeatRequest) (roundtable.View, error)
	RoundtableOpenRound(context.Context, RoundtableOpenRoundRequest) (roundtable.View, error)
	RoundtableProposeMessage(context.Context, RoundtableProposeMessageRequest) (roundtable.View, error)
	RoundtableRelayMessage(context.Context, RoundtableRelayMessageRequest) (roundtable.View, error)
	RoundtableAcknowledgeMessage(context.Context, RoundtableAckMessageRequest) (roundtable.View, error)
	RoundtableInsertMessage(context.Context, RoundtableInsertMessageRequest) (roundtable.View, error)
	RoundtableConcludeSession(context.Context, RoundtableConcludeRequest) (roundtable.View, error)
	RoundtableReadView(context.Context, string) (roundtable.View, error)
}

// roundtableClientFrom returns the read client's RoundtableClient surface, or nil.
func roundtableClientFrom(client ReadClient) RoundtableClient {
	roundtableClient, _ := client.(RoundtableClient)
	return roundtableClient
}

func (client *DaemonReadClient) RoundtableCreateSession(
	ctx context.Context,
	request RoundtableSessionCreateRequest,
) (roundtable.View, error) {
	var view roundtable.View
	err := client.client.Call(ctx, "roundtable_session_create", request, &view)
	return view, err
}

func (client *DaemonReadClient) RoundtableAddSeat(
	ctx context.Context,
	request RoundtableAddSeatRequest,
) (roundtable.View, error) {
	var view roundtable.View
	err := client.client.Call(ctx, "roundtable_add_seat", request, &view)
	return view, err
}

func (client *DaemonReadClient) RoundtableOpenRound(
	ctx context.Context,
	request RoundtableOpenRoundRequest,
) (roundtable.View, error) {
	var view roundtable.View
	err := client.client.Call(ctx, "roundtable_open_round", request, &view)
	return view, err
}

func (client *DaemonReadClient) RoundtableProposeMessage(
	ctx context.Context,
	request RoundtableProposeMessageRequest,
) (roundtable.View, error) {
	var view roundtable.View
	err := client.client.Call(ctx, "roundtable_propose_message", request, &view)
	return view, err
}

func (client *DaemonReadClient) RoundtableRelayMessage(
	ctx context.Context,
	request RoundtableRelayMessageRequest,
) (roundtable.View, error) {
	var view roundtable.View
	err := client.client.Call(ctx, "roundtable_relay_message", request, &view)
	return view, err
}

func (client *DaemonReadClient) RoundtableAcknowledgeMessage(
	ctx context.Context,
	request RoundtableAckMessageRequest,
) (roundtable.View, error) {
	var view roundtable.View
	err := client.client.Call(ctx, "roundtable_ack_message", request, &view)
	return view, err
}

func (client *DaemonReadClient) RoundtableInsertMessage(
	ctx context.Context,
	request RoundtableInsertMessageRequest,
) (roundtable.View, error) {
	var view roundtable.View
	err := client.client.Call(ctx, "roundtable_insert_message", request, &view)
	return view, err
}

func (client *DaemonReadClient) RoundtableConcludeSession(
	ctx context.Context,
	request RoundtableConcludeRequest,
) (roundtable.View, error) {
	var view roundtable.View
	err := client.client.Call(ctx, "roundtable_conclude", request, &view)
	return view, err
}

func (client *DaemonReadClient) RoundtableReadView(
	ctx context.Context,
	sessionID string,
) (roundtable.View, error) {
	var view roundtable.View
	err := client.client.Call(ctx, "roundtable_snapshot", RoundtableSnapshotRequest{
		SchemaVersion: 1, SessionID: sessionID,
	}, &view)
	return view, err
}

// Wire request shapes mirroring the daemon's strict IPC DTOs. The daemon
// stamps EmittedAt server-side, so clients send only identities + correlation.
type RoundtableSessionCreateRequest struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	ModeratorSeat string `json:"moderator_seat"`
	Title         string `json:"title"`
	CorrelationID string `json:"correlation_id"`
}

type RoundtableAddSeatRequest struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	SeatID        string `json:"seat_id"`
	DisplayName   string `json:"display_name"`
	CorrelationID string `json:"correlation_id"`
}

type RoundtableOpenRoundRequest struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	RoundID       string `json:"round_id"`
	ModeratorSeat string `json:"moderator_seat"`
	CorrelationID string `json:"correlation_id"`
}

type RoundtableProposeMessageRequest struct {
	SchemaVersion int      `json:"schema_version"`
	SessionID     string   `json:"session_id"`
	RoundID       string   `json:"round_id"`
	MessageID     string   `json:"message_id"`
	WriterSeat    string   `json:"writer_seat"`
	TargetSeat    string   `json:"target_seat"`
	Body          string   `json:"body"`
	ArtifactRefs  []string `json:"artifact_refs"`
	CorrelationID string   `json:"correlation_id"`
}

type RoundtableRelayMessageRequest struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	MessageID     string `json:"message_id"`
	ModeratorSeat string `json:"moderator_seat"`
	CorrelationID string `json:"correlation_id"`
}

type RoundtableAckMessageRequest struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	MessageID     string `json:"message_id"`
	SeatID        string `json:"seat_id"`
	CorrelationID string `json:"correlation_id"`
}

type RoundtableInsertMessageRequest struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	MessageID     string `json:"message_id"`
	ModeratorSeat string `json:"moderator_seat"`
	CorrelationID string `json:"correlation_id"`
}

type RoundtableConcludeRequest struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	ModeratorSeat string `json:"moderator_seat"`
	CorrelationID string `json:"correlation_id"`
}

type RoundtableSnapshotRequest struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
}

// Dual-seat journey identities. The moderator hosts a writer and a target
// seat and confirms each hop (propose -> relay -> acknowledge -> insert) before
// concluding with a digest-bound AlignmentSummary.
const (
	roundtableModeratorSeat  = "seat-moderator"
	roundtableWriterSeat     = "seat-writer"
	roundtableTargetSeat     = "seat-target"
	roundtableRoundID        = "round-1"
	roundtableMessageID      = "msg-1"
	roundtableDemoTitle      = "Roundtable handoff"
	roundtableDemoBody       = "Governed handoff: bounded diagnosis for the dual-seat journey."
	roundtableDefaultSession = "rt-tui"
)

type roundtableStep int

const (
	roundtableStepNone roundtableStep = iota
	roundtableStepCreateSession
	roundtableStepAddSeats
	roundtableStepOpenRound
	roundtableStepPropose
	roundtableStepRelay
	roundtableStepAcknowledge
	roundtableStepInsert
	roundtableStepConclude
	roundtableStepConcluded
)

func (step roundtableStep) label() string {
	switch step {
	case roundtableStepCreateSession:
		return "create session"
	case roundtableStepAddSeats:
		return "add writer + target seats"
	case roundtableStepOpenRound:
		return "open round"
	case roundtableStepPropose:
		return "propose (writer)"
	case roundtableStepRelay:
		return "relay (moderator)"
	case roundtableStepAcknowledge:
		return "acknowledge (target)"
	case roundtableStepInsert:
		return "insert (moderator)"
	case roundtableStepConclude:
		return "conclude (moderator)"
	case roundtableStepConcluded:
		return "session concluded"
	default:
		return ""
	}
}

// roundtableCurrentStep derives the next journey step from the replayed view.
// Because the view is rebuilt from the Journal, restarting the TUI and
// refreshing yields the same step and never double-continues a completed hop.
func roundtableCurrentStep(view roundtable.View, found bool) roundtableStep {
	if !found {
		return roundtableStepCreateSession
	}
	if view.Session.Concluded {
		return roundtableStepConcluded
	}
	if len(view.Seats) < 3 {
		return roundtableStepAddSeats
	}
	if len(view.Rounds) == 0 {
		return roundtableStepOpenRound
	}
	message, ok := view.Messages[roundtableMessageID]
	if !ok {
		return roundtableStepPropose
	}
	switch message.Status {
	case roundtable.MessagePending:
		return roundtableStepRelay
	case roundtable.MessageRelayed:
		return roundtableStepAcknowledge
	case roundtable.MessageAcknowledged:
		return roundtableStepInsert
	default:
		return roundtableStepConclude
	}
}

func (model Model) loadRoundtable() tea.Cmd {
	client, ctx := model.roundtableClient, model.ctx
	sessionID := model.roundtableSessionID
	return func() tea.Msg {
		if client == nil || sessionID == "" {
			return roundtableLoadedMsg{err: localipc.ErrLocalProductUnavailable}
		}
		view, err := client.RoundtableReadView(ctx, sessionID)
		if err != nil {
			return roundtableLoadedMsg{err: err}
		}
		return roundtableLoadedMsg{view: view}
	}
}

// roundtableAdvanceStep performs the next undone journey hop. It returns the
// tea.Cmd that runs the IPC write and then reloads the authoritative view.
func (model Model) roundtableAdvanceStep() tea.Cmd {
	client, ctx := model.roundtableClient, model.ctx
	sessionID := model.roundtableSessionID
	return func() tea.Msg {
		if client == nil {
			return roundtableCommandDoneMsg{err: localipc.ErrLocalProductUnavailable}
		}
		if sessionID == "" {
			return roundtableCommandDoneMsg{err: roundtable.ErrRoundtableSessionNotFound}
		}
		view := model.roundtableView
		found := view.Session.ID != ""
		var writeErr error
		nextCorrelation := roundtableUUID()
		switch roundtableCurrentStep(view, found) {
		case roundtableStepCreateSession:
			_, writeErr = client.RoundtableCreateSession(ctx, RoundtableSessionCreateRequest{
				SchemaVersion: 1, SessionID: sessionID, ModeratorSeat: roundtableModeratorSeat,
				Title: roundtableDemoTitle, CorrelationID: nextCorrelation,
			})
		case roundtableStepAddSeats:
			// Idempotent per seat: a partially-committed AddSeats step (writer
			// committed but target failed) must not re-add the existing seat on
			// retry/restart, otherwise the session wedges on conflict.
			if _, exists := view.Seats[roundtableWriterSeat]; !exists {
				if _, writeErr = client.RoundtableAddSeat(ctx, RoundtableAddSeatRequest{
					SchemaVersion: 1, SessionID: sessionID, SeatID: roundtableWriterSeat,
					DisplayName: "Writer Seat", CorrelationID: nextCorrelation,
				}); writeErr != nil {
					break
				}
			}
			if _, exists := view.Seats[roundtableTargetSeat]; !exists {
				_, writeErr = client.RoundtableAddSeat(ctx, RoundtableAddSeatRequest{
					SchemaVersion: 1, SessionID: sessionID, SeatID: roundtableTargetSeat,
					DisplayName: "Target Seat", CorrelationID: roundtableUUID(),
				})
			}
		case roundtableStepOpenRound:
			_, writeErr = client.RoundtableOpenRound(ctx, RoundtableOpenRoundRequest{
				SchemaVersion: 1, SessionID: sessionID, RoundID: roundtableRoundID,
				ModeratorSeat: roundtableModeratorSeat, CorrelationID: nextCorrelation,
			})
		case roundtableStepPropose:
			_, writeErr = client.RoundtableProposeMessage(ctx, RoundtableProposeMessageRequest{
				SchemaVersion: 1, SessionID: sessionID, RoundID: roundtableRoundID,
				MessageID: roundtableMessageID, WriterSeat: roundtableWriterSeat,
				TargetSeat: roundtableTargetSeat, Body: roundtableDemoBody,
				ArtifactRefs: []string{}, CorrelationID: nextCorrelation,
			})
		case roundtableStepRelay:
			_, writeErr = client.RoundtableRelayMessage(ctx, RoundtableRelayMessageRequest{
				SchemaVersion: 1, SessionID: sessionID, MessageID: roundtableMessageID,
				ModeratorSeat: roundtableModeratorSeat, CorrelationID: nextCorrelation,
			})
		case roundtableStepAcknowledge:
			_, writeErr = client.RoundtableAcknowledgeMessage(ctx, RoundtableAckMessageRequest{
				SchemaVersion: 1, SessionID: sessionID, MessageID: roundtableMessageID,
				SeatID: roundtableTargetSeat, CorrelationID: nextCorrelation,
			})
		case roundtableStepInsert:
			_, writeErr = client.RoundtableInsertMessage(ctx, RoundtableInsertMessageRequest{
				SchemaVersion: 1, SessionID: sessionID, MessageID: roundtableMessageID,
				ModeratorSeat: roundtableModeratorSeat, CorrelationID: nextCorrelation,
			})
		case roundtableStepConclude:
			_, writeErr = client.RoundtableConcludeSession(ctx, RoundtableConcludeRequest{
				SchemaVersion: 1, SessionID: sessionID,
				ModeratorSeat: roundtableModeratorSeat, CorrelationID: nextCorrelation,
			})
		default:
			return roundtableCommandDoneMsg{}
		}
		if writeErr != nil {
			return roundtableCommandDoneMsg{err: writeErr}
		}
		return roundtableCommandDoneMsg{}
	}
}

func (model Model) renderRoundtableView() string {
	lines := []string{
		"Roundtable · Governed Handoff · Journal-authoritative",
		"n next step · r refresh · e session · q quit",
	}
	if model.roundtableClient == nil {
		lines = append(lines, "Roundtable service unavailable")
		return strings.Join(lines, "\n") + "\n"
	}
	if model.roundtableError != "" {
		lines = append(lines, styleSection("Roundtable error"))
		lines = append(lines, sanitizeCell(model.roundtableError, 88))
	}
	if model.roundtableSessionID == "" {
		lines = append(lines, "No session yet · n to create (default "+roundtableDefaultSession+")")
		return strings.Join(lines, "\n") + "\n"
	}
	sessionID := model.roundtableSessionID
	lines = append(lines, "Session · "+sanitizeCell(sessionID, 64))
	view, found := model.roundtableView, true
	if view.Session.ID == "" {
		found = false
	}
	step := roundtableCurrentStep(view, found)
	lines = append(lines, "Next step · "+step.label())
	if !found {
		lines = append(lines, "Session not materialized yet · r to refresh")
		return strings.Join(lines, "\n") + "\n"
	}
	if view.Session.Concluded {
		lines = append(lines, styleSection("AlignmentSummary"))
		lines = append(
			lines,
			"Session concluded · digest "+sanitizeCell(view.Digest, 64),
		)
	}
	lines = append(lines, styleSection("Seats"))
	for _, seat := range view.SeatsList() {
		marker := "•"
		if !seat.Available {
			marker = "×"
		}
		role := ""
		switch seat.ID {
		case roundtableModeratorSeat:
			role = " moderator"
		case roundtableWriterSeat:
			role = " writer"
		case roundtableTargetSeat:
			role = " target"
		}
		lines = append(lines, fmt.Sprintf(
			"%s %s · %s%s", marker,
			sanitizeCell(seat.ID, 24),
			sanitizeCell(seat.DisplayName, 28), role,
		))
	}
	if len(view.Rounds) > 0 {
		lines = append(lines, styleSection("Rounds"))
		for _, round := range view.Rounds {
			lines = append(lines, fmt.Sprintf(
				"• %s · seq %d · %d message(s)",
				sanitizeCell(round.ID, 24), round.Sequence, round.MessageCount,
			))
		}
	}
	if len(view.Messages) > 0 {
		lines = append(lines, styleSection("Messages"))
		for _, message := range view.Messages {
			lines = append(lines, fmt.Sprintf(
				"• %s %s → %s · %s",
				sanitizeCell(message.ID, 16),
				sanitizeCell(message.WriterSeat, 16),
				sanitizeCell(message.TargetSeat, 16),
				styleStatus(humanizeStatus(message.Status)),
			))
			lines = append(lines, "    "+sanitizeCell(message.Body, 76))
			lines = append(lines, "    body digest "+sanitizeCell(message.BodyDigest, 64))
		}
	}
	if step == roundtableStepConcluded {
		lines = append(
			lines,
			"Concluded · repeat n does nothing · r rebuilds the same view",
		)
	}
	return strings.Join(lines, "\n") + "\n"
}

func roundtableUUID() string {
	var idBytes [16]byte
	if _, err := rand.Read(idBytes[:]); err != nil {
		return "00000000-0000-4000-8000-000000000000"
	}
	idBytes[6] = (idBytes[6] & 0x0f) | 0x40
	idBytes[8] = (idBytes[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(idBytes[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]
}

// RunRoundtableLiveJourney drives the Roundtable dual-seat journey on a fresh
// TUI model against the given daemon client: create session, add writer +
// target seats, open a round, propose, relay, acknowledge, insert, conclude.
// It returns the rendered Roundtable screen after every hop plus the final
// authoritative view. This is the programmatic equivalent of pressing `n` in
// the Roundtable screen and is used by installed-live gates.
func RunRoundtableLiveJourney(
	ctx context.Context,
	client ReadClient,
	sessionID string,
) ([]string, roundtable.View, error) {
	if ctx == nil || client == nil || sessionID == "" {
		return nil, roundtable.View{}, roundtable.ErrInvalidRoundtableSession
	}
	model, err := newModelWithContext(ctx, client)
	if err != nil {
		return nil, roundtable.View{}, err
	}
	if model.roundtableClient == nil {
		return nil, roundtable.View{}, localipc.ErrLocalProductUnavailable
	}
	model.roundtableSessionID = sessionID
	model.screenIndex = indexOfScreen(ScreenRoundtable)
	var frames []string
	step := roundtableCurrentStep(model.roundtableView, false)
	for guard := 0; guard < 12; guard++ {
		frames = append(frames, model.renderRoundtableView())
		if step == roundtableStepConcluded || step == roundtableStepConclude &&
			model.roundtableView.Session.Concluded {
			break
		}
		updated, reload := model.Update(model.roundtableAdvanceStep()())
		model = updated.(Model)
		for i := 0; i < 4 && reload != nil; i++ {
			updated, reload = model.Update(reload())
			model = updated.(Model)
		}
		step = roundtableCurrentStep(model.roundtableView, model.roundtableView.Session.ID != "")
	}
	frames = append(frames, model.renderRoundtableView())
	if !model.roundtableView.Session.Concluded {
		return frames, model.roundtableView, roundtable.ErrRoundtableConflict
	}
	return frames, model.roundtableView, nil
}
