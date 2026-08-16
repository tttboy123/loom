package main

import (
	"context"
	"errors"
	"time"

	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/roundtable"
)

// productRoundtableRoute is the governed-handoff ledger route exposed over
// Local IPC. Every command is validated against the replayed Journal stream
// and appended through the existing Journal CAS; Conclude additionally
// publishes a digest-bound AlignmentSummary Artifact to the Evidence Store.
type productRoundtableRoute interface {
	CreateSession(context.Context, roundtable.CreateSessionCommand) (roundtable.View, error)
	AddSeat(context.Context, roundtable.AddSeatCommand) (roundtable.View, error)
	RetireSeat(context.Context, roundtable.RetireSeatCommand) (roundtable.View, error)
	OpenRound(context.Context, roundtable.OpenRoundCommand) (roundtable.View, error)
	ProposeMessage(context.Context, roundtable.ProposeMessageCommand) (roundtable.View, error)
	RelayMessage(context.Context, roundtable.RelayMessageCommand) (roundtable.View, error)
	AcknowledgeMessage(context.Context, roundtable.AcknowledgeMessageCommand) (roundtable.View, error)
	InsertMessage(context.Context, roundtable.InsertMessageCommand) (roundtable.View, error)
	DropMessage(context.Context, roundtable.DropMessageCommand) (roundtable.View, error)
	ConcludeSession(context.Context, roundtable.ConcludeSessionCommand) (roundtable.View, error)
	ReadView(context.Context, string) (roundtable.View, error)
}

var _ productRoundtableRoute = (*productRoundtableController)(nil)

// productRoundtableController stamps server-authoritative timestamps and
// delegates to the journal-backed roundtable.Authority.
type productRoundtableController struct {
	authority *roundtable.Authority
	now       func() time.Time
}

func newProductRoundtableController(
	authority *roundtable.Authority,
	now func() time.Time,
) (*productRoundtableController, error) {
	if authority == nil || now == nil {
		return nil, roundtable.ErrInvalidRoundtableSession
	}
	return &productRoundtableController{
		authority: authority, now: now,
	}, nil
}

func (controller *productRoundtableController) stamp() time.Time {
	return controller.now()
}

func (controller *productRoundtableController) CreateSession(
	ctx context.Context,
	command roundtable.CreateSessionCommand,
) (roundtable.View, error) {
	command.EmittedAt = controller.stamp()
	return controller.authority.CreateSession(ctx, command)
}

func (controller *productRoundtableController) AddSeat(
	ctx context.Context,
	command roundtable.AddSeatCommand,
) (roundtable.View, error) {
	command.EmittedAt = controller.stamp()
	return controller.authority.AddSeat(ctx, command)
}

func (controller *productRoundtableController) RetireSeat(
	ctx context.Context,
	command roundtable.RetireSeatCommand,
) (roundtable.View, error) {
	command.EmittedAt = controller.stamp()
	return controller.authority.RetireSeat(ctx, command)
}

func (controller *productRoundtableController) OpenRound(
	ctx context.Context,
	command roundtable.OpenRoundCommand,
) (roundtable.View, error) {
	command.EmittedAt = controller.stamp()
	return controller.authority.OpenRound(ctx, command)
}

func (controller *productRoundtableController) ProposeMessage(
	ctx context.Context,
	command roundtable.ProposeMessageCommand,
) (roundtable.View, error) {
	command.EmittedAt = controller.stamp()
	return controller.authority.ProposeMessage(ctx, command)
}

func (controller *productRoundtableController) RelayMessage(
	ctx context.Context,
	command roundtable.RelayMessageCommand,
) (roundtable.View, error) {
	command.EmittedAt = controller.stamp()
	return controller.authority.RelayMessage(ctx, command)
}

func (controller *productRoundtableController) AcknowledgeMessage(
	ctx context.Context,
	command roundtable.AcknowledgeMessageCommand,
) (roundtable.View, error) {
	command.EmittedAt = controller.stamp()
	return controller.authority.AcknowledgeMessage(ctx, command)
}

func (controller *productRoundtableController) InsertMessage(
	ctx context.Context,
	command roundtable.InsertMessageCommand,
) (roundtable.View, error) {
	command.EmittedAt = controller.stamp()
	return controller.authority.InsertMessage(ctx, command)
}

func (controller *productRoundtableController) DropMessage(
	ctx context.Context,
	command roundtable.DropMessageCommand,
) (roundtable.View, error) {
	command.EmittedAt = controller.stamp()
	return controller.authority.DropMessage(ctx, command)
}

func (controller *productRoundtableController) ConcludeSession(
	ctx context.Context,
	command roundtable.ConcludeSessionCommand,
) (roundtable.View, error) {
	command.EmittedAt = controller.stamp()
	return controller.authority.ConcludeSession(ctx, command)
}

func (controller *productRoundtableController) ReadView(
	ctx context.Context,
	sessionID string,
) (roundtable.View, error) {
	return controller.authority.ReadView(ctx, sessionID)
}

// productRoundtableRequest is the strict IPC envelope shared by all
// roundtable methods; each method decodes its own params below. The daemon
// stamps EmittedAt server-side so clients cannot forge fact timestamps.
type productRoundtableSessionCreateParams struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	ModeratorSeat string `json:"moderator_seat"`
	Title         string `json:"title"`
	CorrelationID string `json:"correlation_id"`
}

type productRoundtableAddSeatParams struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	SeatID        string `json:"seat_id"`
	DisplayName   string `json:"display_name"`
	CorrelationID string `json:"correlation_id"`
}

type productRoundtableRetireSeatParams struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	SeatID        string `json:"seat_id"`
	ModeratorSeat string `json:"moderator_seat"`
	CorrelationID string `json:"correlation_id"`
}

type productRoundtableOpenRoundParams struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	RoundID       string `json:"round_id"`
	ModeratorSeat string `json:"moderator_seat"`
	CorrelationID string `json:"correlation_id"`
}

type productRoundtableProposeMessageParams struct {
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

type productRoundtableRelayMessageParams struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	MessageID     string `json:"message_id"`
	ModeratorSeat string `json:"moderator_seat"`
	CorrelationID string `json:"correlation_id"`
}

type productRoundtableAckMessageParams struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	MessageID     string `json:"message_id"`
	SeatID        string `json:"seat_id"`
	CorrelationID string `json:"correlation_id"`
}

type productRoundtableInsertMessageParams struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	MessageID     string `json:"message_id"`
	ModeratorSeat string `json:"moderator_seat"`
	CorrelationID string `json:"correlation_id"`
}

type productRoundtableDropMessageParams struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	MessageID     string `json:"message_id"`
	ModeratorSeat string `json:"moderator_seat"`
	CorrelationID string `json:"correlation_id"`
}

type productRoundtableConcludeParams struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	ModeratorSeat string `json:"moderator_seat"`
	CorrelationID string `json:"correlation_id"`
}

type productRoundtableSnapshotParams struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
}

func validProductRoundtableSchemaVersion(value int) bool {
	return value == roundtable.SchemaVersion
}

func productRoundtableMethod(method string) bool {
	switch method {
	case "roundtable_session_create", "roundtable_add_seat", "roundtable_retire_seat",
		"roundtable_open_round", "roundtable_propose_message", "roundtable_relay_message",
		"roundtable_ack_message", "roundtable_insert_message", "roundtable_drop_message",
		"roundtable_conclude", "roundtable_snapshot":
		return true
	default:
		return false
	}
}

func productRoundtableServiceError(err error) localipc.Response {
	switch {
	case errors.Is(err, roundtable.ErrRoundtableSessionNotFound),
		errors.Is(err, roundtable.ErrRoundtableSeatNotFound),
		errors.Is(err, roundtable.ErrRoundtableMessageNotFound),
		errors.Is(err, roundtable.ErrRoundtableRoundNotFound):
		return productErrorResponse("not_found", err)
	case errors.Is(err, roundtable.ErrRoundtableNotModerator):
		return productErrorResponse("not_moderator", err)
	case errors.Is(err, roundtable.ErrRoundtableSeatUnavailable):
		return productErrorResponse("seat_unavailable", err)
	case errors.Is(err, roundtable.ErrRoundtableAlreadyConcluded):
		return productErrorResponse("concluded", err)
	case errors.Is(err, roundtable.ErrRoundtableInvalidBody):
		return productErrorResponse("invalid_body", err)
	case errors.Is(err, roundtable.ErrRoundtableInvalidDigest):
		return productErrorResponse("invalid_digest", err)
	case errors.Is(err, roundtable.ErrRoundtableDigestMismatch):
		return productErrorResponse("digest_mismatch", err)
	case errors.Is(err, roundtable.ErrRoundtableTooManyMessages):
		return productErrorResponse("too_many_messages", err)
	case errors.Is(err, roundtable.ErrRoundtableTooManySeats):
		return productErrorResponse("too_many_seats", err)
	case errors.Is(err, roundtable.ErrRoundtableConflict):
		return productErrorResponse("conflict", err)
	case errors.Is(err, roundtable.ErrInvalidRoundtableSession),
		errors.Is(err, roundtable.ErrInvalidRoundtableSeat),
		errors.Is(err, roundtable.ErrInvalidRoundtableMessage):
		return productErrorResponse("invalid_request", err)
	default:
		return productServiceError(err)
	}
}
