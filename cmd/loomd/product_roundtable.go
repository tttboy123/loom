package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"loom-pi-rebuild/internal/agentinbox"
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
	PauseRound(context.Context, roundtable.PauseRoundCommand) (roundtable.View, error)
	SteerSeat(context.Context, productRoundtableSteerSeatCommand) (roundtable.View, error)
	RetrySeat(context.Context, productRoundtableRetrySeatCommand) (roundtable.View, error)
	SkipSeat(context.Context, roundtable.SkipSeatCommand) (roundtable.View, error)
	ReplaceSeat(context.Context, productRoundtableReplaceSeatCommand) (roundtable.View, error)
	ExportSession(context.Context, productRoundtableExportCommand) (roundtable.ExportDocumentResult, error)
	ImportSession(context.Context, productRoundtableImportCommand) (roundtable.ImportResult, error)
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
	authority       *roundtable.Authority
	bindingResolver productRoundtableBindingResolver
	execution       *productRoundtableExecution
	agentInput      productAgentInputRoute
	now             func() time.Time
	inputReadyDelay time.Duration
	inputReadyWait  time.Duration
}

const (
	productRoundtableInputReadyDelay = 200 * time.Millisecond
	productRoundtableInputReadyWait  = 2 * time.Minute
)

func (controller *productRoundtableController) SetAgentInput(
	agentInput productAgentInputRoute,
) error {
	if controller == nil || agentInput == nil || controller.agentInput != nil {
		return roundtable.ErrInvalidRoundtableIntervention
	}
	controller.agentInput = agentInput
	return nil
}

func (controller *productRoundtableController) SetExecution(
	execution *productRoundtableExecution,
) error {
	if controller == nil || execution == nil || controller.execution != nil {
		return roundtable.ErrInvalidRoundtableSeatAttempt
	}
	controller.execution = execution
	return nil
}

type productRoundtableBindingResolver interface {
	ResolveSessionContext(
		context.Context,
		roundtable.SessionLinkRequest,
	) (roundtable.SessionContext, error)
	ResolveSeatBinding(
		context.Context,
		string,
		string,
		roundtable.SessionContext,
		roundtable.SeatBindingRequest,
		int,
	) (roundtable.FrozenSeatBinding, error)
}

func newProductRoundtableController(
	authority *roundtable.Authority,
	now func() time.Time,
	bindingResolver productRoundtableBindingResolver,
) (*productRoundtableController, error) {
	if authority == nil || now == nil {
		return nil, roundtable.ErrInvalidRoundtableSession
	}
	return &productRoundtableController{
		authority: authority, bindingResolver: bindingResolver, now: now,
		inputReadyDelay: productRoundtableInputReadyDelay,
		inputReadyWait:  productRoundtableInputReadyWait,
	}, nil
}

func (controller *productRoundtableController) stamp() time.Time {
	return controller.now().UTC()
}

func (controller *productRoundtableController) CreateSession(
	ctx context.Context,
	command roundtable.CreateSessionCommand,
) (roundtable.View, error) {
	if command.Link != nil {
		if command.Context != nil || controller.bindingResolver == nil {
			return roundtable.View{}, roundtable.ErrInvalidRoundtableSession
		}
		resolved, err := controller.bindingResolver.ResolveSessionContext(ctx, *command.Link)
		if err != nil {
			return roundtable.View{}, err
		}
		command.Link = nil
		command.Context = &resolved
	}
	command.EmittedAt = controller.stamp()
	return controller.authority.CreateSession(ctx, command)
}

func (controller *productRoundtableController) AddSeat(
	ctx context.Context,
	command roundtable.AddSeatCommand,
) (roundtable.View, error) {
	if command.Selection != nil {
		if command.Binding != nil || controller.bindingResolver == nil {
			return roundtable.View{}, roundtable.ErrInvalidRoundtableSeatBinding
		}
		view, err := controller.authority.ReadView(ctx, command.SessionID)
		if err != nil {
			return roundtable.View{}, err
		}
		if view.Session.Context == nil {
			return roundtable.View{}, roundtable.ErrInvalidRoundtableSeatBinding
		}
		revision := 1
		if existing, found := view.Seats[command.SeatID]; found && existing.Binding != nil {
			revision = existing.Binding.MembershipRevision
			if !existing.Available {
				revision++
			}
		}
		resolved, resolveErr := controller.bindingResolver.ResolveSeatBinding(
			ctx, command.SessionID, command.SeatID, *view.Session.Context,
			*command.Selection, revision,
		)
		if resolveErr != nil {
			return roundtable.View{}, resolveErr
		}
		command.Selection = nil
		command.Binding = &resolved
	}
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
	prompt := strings.TrimSpace(command.Prompt)
	if command.Prompt != prompt || len(prompt) > roundtable.MaxRoundPromptBytes ||
		!utf8.ValidString(prompt) || strings.ContainsAny(prompt, "\x00\r") {
		return roundtable.View{}, roundtable.ErrRoundtableInvalidBody
	}
	if prompt == "" && controller.execution != nil {
		current, err := controller.authority.ReadView(ctx, command.SessionID)
		if err != nil {
			return roundtable.View{}, err
		}
		if current.Session.Context != nil {
			return roundtable.View{}, roundtable.ErrInvalidRoundtableSeatAttempt
		}
	}
	command.Prompt = ""
	command.EmittedAt = controller.stamp()
	view, err := controller.authority.OpenRound(ctx, command)
	if err != nil || controller.execution == nil || view.Session.Context == nil ||
		prompt == "" {
		return view, err
	}
	return controller.execution.StartRound(ctx, view, prompt, command.CorrelationID)
}

func (controller *productRoundtableController) PauseRound(
	ctx context.Context,
	command roundtable.PauseRoundCommand,
) (roundtable.View, error) {
	command.EmittedAt = controller.stamp()
	_, err := controller.authority.PauseRound(ctx, command)
	if err != nil {
		return roundtable.View{}, err
	}
	if controller.execution != nil {
		controller.execution.CancelRound(command.SessionID, command.RoundID)
	}
	return controller.ReadView(ctx, command.SessionID)
}

type productRoundtableSteerSeatCommand struct {
	SessionID      string
	RoundID        string
	InterventionID string
	ModeratorSeat  string
	SeatID         string
	AttemptID      string
	Guidance       []byte
	CorrelationID  string
}

func (controller *productRoundtableController) SteerSeat(
	ctx context.Context,
	command productRoundtableSteerSeatCommand,
) (roundtable.View, error) {
	defer clearProductAgentInput(command.Guidance)
	if controller.agentInput == nil || len(command.Guidance) == 0 ||
		len(command.Guidance) > roundtable.MaxRoundPromptBytes {
		return roundtable.View{}, roundtable.ErrInvalidRoundtableIntervention
	}
	view, err := controller.authority.ReadView(ctx, command.SessionID)
	if err != nil {
		return roundtable.View{}, err
	}
	digest := sha256.Sum256(command.Guidance)
	contentDigest := hex.EncodeToString(digest[:])
	if existing, found := view.Interventions[command.InterventionID]; found {
		if existing.Kind == roundtable.InterventionSteer &&
			existing.RoundID == command.RoundID &&
			existing.SeatID == command.SeatID &&
			existing.AttemptID == command.AttemptID &&
			existing.ContentDigest == contentDigest {
			return view, nil
		}
		return roundtable.View{}, roundtable.ErrRoundtableInterventionConflict
	}
	attempt, found := view.Attempts[command.AttemptID]
	if !found || attempt.RoundID != command.RoundID || attempt.SeatID != command.SeatID ||
		attempt.Status != roundtable.SeatAttemptRunning {
		return roundtable.View{}, roundtable.ErrInvalidRoundtableIntervention
	}
	requestedAt := controller.stamp()
	receipt, err := controller.admitSteerWhenReady(ctx, command, attempt)
	if err != nil {
		return roundtable.View{}, err
	}
	return controller.authority.RecordSeatSteer(ctx, roundtable.RecordSeatSteerCommand{
		SessionID: command.SessionID, RoundID: command.RoundID,
		InterventionID: command.InterventionID, ModeratorSeat: command.ModeratorSeat,
		SeatID: command.SeatID, AttemptID: command.AttemptID, InputID: receipt.InputID,
		ContentDigest: contentDigest, EmittedAt: requestedAt,
		CorrelationID: command.CorrelationID,
	})
}

func (controller *productRoundtableController) admitSteerWhenReady(
	ctx context.Context,
	command productRoundtableSteerSeatCommand,
	attempt roundtable.SeatAttempt,
) (productAgentInputReceipt, error) {
	deadline := time.Now().Add(controller.inputReadyWait)
	for {
		content := append([]byte(nil), command.Guidance...)
		receipt, err := func() (productAgentInputReceipt, error) {
			defer clearProductAgentInput(content)
			return controller.agentInput.AdmitAgentInput(ctx, productAgentInputRequest{
				SchemaVersion: productAgentInputSchemaVersion,
				SegmentID:     attempt.SegmentID, AgentInstanceID: attempt.AgentInstanceID,
				WorkItemID: attempt.WorkItemID, RunID: attempt.RunID,
				ClaimGeneration: attempt.ClaimGeneration, Mode: agentinbox.ModeSteer,
				ContextScope: agentinbox.ScopeAgentPrivate,
				Content:      content, IncidentID: command.CorrelationID,
			})
		}()
		if err == nil {
			return receipt, nil
		}
		if !roundtableSteerInputNotReady(err) || time.Now().Add(controller.inputReadyDelay).After(deadline) {
			return productAgentInputReceipt{}, err
		}
		latest, readErr := controller.authority.ReadView(ctx, command.SessionID)
		if readErr != nil {
			return productAgentInputReceipt{}, readErr
		}
		current, found := latest.Attempts[command.AttemptID]
		if !found || !sameRoundtableSteerAttempt(current, attempt) ||
			current.Status != roundtable.SeatAttemptRunning {
			return productAgentInputReceipt{}, roundtable.ErrInvalidRoundtableIntervention
		}
		timer := time.NewTimer(controller.inputReadyDelay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return productAgentInputReceipt{}, ctx.Err()
		case <-timer.C:
		}
	}
}

func sameRoundtableSteerAttempt(current, expected roundtable.SeatAttempt) bool {
	return current.AttemptID == expected.AttemptID && current.RoundID == expected.RoundID &&
		current.SeatID == expected.SeatID && current.AttemptNumber == expected.AttemptNumber &&
		current.ExecutionTeamID == expected.ExecutionTeamID &&
		current.WorkItemID == expected.WorkItemID && current.RunID == expected.RunID &&
		current.SegmentID == expected.SegmentID &&
		current.ClaimGeneration == expected.ClaimGeneration &&
		current.RuntimeInstanceID == expected.RuntimeInstanceID &&
		current.AgentInstanceID == expected.AgentInstanceID &&
		current.MembershipRevision == expected.MembershipRevision &&
		current.SeatBindingDigest == expected.SeatBindingDigest &&
		current.ExecutionBindingDigest == expected.ExecutionBindingDigest &&
		current.ContextCapsuleDigest == expected.ContextCapsuleDigest
}

func roundtableSteerInputNotReady(err error) bool {
	return errors.Is(err, errProductActiveAttemptNotFound) ||
		errors.Is(err, errProductAgentInputConflict)
}

type productRoundtableRetrySeatCommand struct {
	SessionID                  string
	RoundID                    string
	InterventionID             string
	ModeratorSeat              string
	SeatID                     string
	AttemptID                  string
	ExpectedMembershipRevision int
	ExpectedSeatBindingDigest  string
	Guidance                   string
	CorrelationID              string
}

func (controller *productRoundtableController) RetrySeat(
	ctx context.Context,
	command productRoundtableRetrySeatCommand,
) (roundtable.View, error) {
	guidance := strings.TrimSpace(command.Guidance)
	if controller.execution == nil || guidance != command.Guidance || guidance == "" ||
		len(guidance) > roundtable.MaxRoundPromptBytes || !utf8.ValidString(guidance) ||
		strings.ContainsAny(guidance, "\x00\r") {
		return roundtable.View{}, roundtable.ErrInvalidRoundtableIntervention
	}
	view, err := controller.authority.ReadView(ctx, command.SessionID)
	if err != nil {
		return roundtable.View{}, err
	}
	attempt, found := view.Attempts[command.AttemptID]
	if !found || attempt.RoundID != command.RoundID || attempt.SeatID != command.SeatID {
		return roundtable.View{}, roundtable.ErrInvalidRoundtableIntervention
	}
	if command.ModeratorSeat != view.Session.ModeratorSeat {
		return roundtable.View{}, roundtable.ErrRoundtableNotModerator
	}
	next := attempt.AttemptNumber + 1
	view, err = controller.authority.RecordSeatRetry(ctx, roundtable.RecordSeatRetryCommand{
		SessionID: command.SessionID, RoundID: command.RoundID,
		InterventionID: command.InterventionID, ModeratorSeat: command.ModeratorSeat,
		SeatID: command.SeatID, AttemptID: command.AttemptID,
		RequestedAttemptNumber:     next,
		ExpectedMembershipRevision: command.ExpectedMembershipRevision,
		ExpectedSeatBindingDigest:  command.ExpectedSeatBindingDigest,
		EmittedAt:                  controller.stamp(),
		CorrelationID:              command.CorrelationID,
	})
	if err != nil {
		return roundtable.View{}, err
	}
	attempt, found = view.Attempts[command.AttemptID]
	if !found {
		return roundtable.View{}, roundtable.ErrInvalidRoundtableIntervention
	}
	for _, existingAttempt := range view.Attempts {
		if existingAttempt.RoundID == command.RoundID &&
			existingAttempt.SeatID == command.SeatID &&
			existingAttempt.AttemptNumber == next {
			return controller.execution.Project(ctx, view), nil
		}
	}
	return controller.execution.StartRetry(ctx, view, command.RoundID, command.SeatID,
		next, attempt, guidance, command.InterventionID, command.CorrelationID)
}

func (controller *productRoundtableController) SkipSeat(
	ctx context.Context,
	command roundtable.SkipSeatCommand,
) (roundtable.View, error) {
	command.EmittedAt = controller.stamp()
	return controller.authority.SkipSeat(ctx, command)
}

type productRoundtableReplaceSeatCommand struct {
	SessionID      string
	RoundID        string
	InterventionID string
	ModeratorSeat  string
	SeatID         string
	DisplayName    string
	Selection      roundtable.SeatBindingRequest
	CorrelationID  string
}

func (controller *productRoundtableController) ReplaceSeat(
	ctx context.Context,
	command productRoundtableReplaceSeatCommand,
) (roundtable.View, error) {
	if controller.bindingResolver == nil {
		return roundtable.View{}, roundtable.ErrInvalidRoundtableSeatBinding
	}
	view, err := controller.authority.ReadView(ctx, command.SessionID)
	if err != nil {
		return roundtable.View{}, err
	}
	seat, found := view.Seats[command.SeatID]
	if !found || seat.Binding == nil || view.Session.Context == nil {
		return roundtable.View{}, roundtable.ErrInvalidRoundtableSeatBinding
	}
	if _, exists := view.Interventions[command.InterventionID]; exists {
		if seat.DisplayName != command.DisplayName ||
			seat.Binding.AgentDefinitionID != command.Selection.AgentDefinitionID ||
			seat.Binding.TeamRoleKind != command.Selection.TeamRoleKind ||
			seat.Binding.RuntimeProfileID != command.Selection.RuntimeProfileID {
			return roundtable.View{}, roundtable.ErrRoundtableInterventionConflict
		}
		return controller.authority.ReplaceSeat(ctx, roundtable.ReplaceSeatCommand{
			SessionID: command.SessionID, RoundID: command.RoundID,
			InterventionID: command.InterventionID, ModeratorSeat: command.ModeratorSeat,
			SeatID: command.SeatID, DisplayName: command.DisplayName, Binding: *seat.Binding,
			EmittedAt: controller.stamp(), CorrelationID: command.CorrelationID,
		})
	}
	binding, err := controller.bindingResolver.ResolveSeatBinding(
		ctx, command.SessionID, command.SeatID, *view.Session.Context,
		command.Selection, seat.Binding.MembershipRevision+1,
	)
	if err != nil {
		return roundtable.View{}, err
	}
	return controller.authority.ReplaceSeat(ctx, roundtable.ReplaceSeatCommand{
		SessionID: command.SessionID, RoundID: command.RoundID,
		InterventionID: command.InterventionID, ModeratorSeat: command.ModeratorSeat,
		SeatID: command.SeatID, DisplayName: command.DisplayName, Binding: binding,
		EmittedAt: controller.stamp(), CorrelationID: command.CorrelationID,
	})
}

type productRoundtableExportCommand struct {
	SessionID     string
	CorrelationID string
}

func (controller *productRoundtableController) ExportSession(
	ctx context.Context,
	command productRoundtableExportCommand,
) (roundtable.ExportDocumentResult, error) {
	if command.SessionID == "" {
		return roundtable.ExportDocumentResult{}, roundtable.ErrInvalidExportContract
	}
	return controller.authority.ExportDocument(
		ctx, command.SessionID, controller.stamp().Add(24*time.Hour), command.CorrelationID,
	)
}

type productRoundtableImportCommand struct {
	Document      []byte
	CorrelationID string
}

func (controller *productRoundtableController) ImportSession(
	ctx context.Context,
	command productRoundtableImportCommand,
) (roundtable.ImportResult, error) {
	return controller.authority.ImportDocument(ctx, command.Document, command.CorrelationID)
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
	view, err := controller.authority.ConcludeSession(ctx, command)
	if err != nil || controller.execution == nil {
		return view, err
	}
	return controller.execution.Project(ctx, view), nil
}

func (controller *productRoundtableController) ReadView(
	ctx context.Context,
	sessionID string,
) (roundtable.View, error) {
	view, err := controller.authority.ReadView(ctx, sessionID)
	if err != nil || controller.execution == nil {
		return view, err
	}
	return controller.execution.Project(ctx, view), nil
}

// productRoundtableRequest is the strict IPC envelope shared by all
// roundtable methods; each method decodes its own params below. The daemon
// stamps EmittedAt server-side so clients cannot forge fact timestamps.
type productRoundtableSessionCreateParams struct {
	SchemaVersion int                            `json:"schema_version"`
	SessionID     string                         `json:"session_id"`
	ModeratorSeat string                         `json:"moderator_seat"`
	Title         string                         `json:"title"`
	Link          *roundtable.SessionLinkRequest `json:"link,omitempty"`
	CorrelationID string                         `json:"correlation_id"`
}

type productRoundtableAddSeatParams struct {
	SchemaVersion int                            `json:"schema_version"`
	SessionID     string                         `json:"session_id"`
	SeatID        string                         `json:"seat_id"`
	DisplayName   string                         `json:"display_name"`
	Selection     *roundtable.SeatBindingRequest `json:"selection,omitempty"`
	CorrelationID string                         `json:"correlation_id"`
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
	Prompt        string `json:"prompt,omitempty"`
	CorrelationID string `json:"correlation_id"`
}

type productRoundtablePauseRoundParams struct {
	SchemaVersion  int    `json:"schema_version"`
	SessionID      string `json:"session_id"`
	RoundID        string `json:"round_id"`
	InterventionID string `json:"intervention_id"`
	ModeratorSeat  string `json:"moderator_seat"`
	CorrelationID  string `json:"correlation_id"`
}

type productRoundtableSteerSeatParams struct {
	SchemaVersion  int    `json:"schema_version"`
	SessionID      string `json:"session_id"`
	RoundID        string `json:"round_id"`
	InterventionID string `json:"intervention_id"`
	ModeratorSeat  string `json:"moderator_seat"`
	SeatID         string `json:"seat_id"`
	AttemptID      string `json:"attempt_id"`
	Guidance       []byte `json:"guidance"`
	CorrelationID  string `json:"correlation_id"`
}

type productRoundtableRetrySeatParams struct {
	SchemaVersion              int    `json:"schema_version"`
	SessionID                  string `json:"session_id"`
	RoundID                    string `json:"round_id"`
	InterventionID             string `json:"intervention_id"`
	ModeratorSeat              string `json:"moderator_seat"`
	SeatID                     string `json:"seat_id"`
	AttemptID                  string `json:"attempt_id"`
	ExpectedMembershipRevision int    `json:"expected_membership_revision,omitempty"`
	ExpectedSeatBindingDigest  string `json:"expected_seat_binding_digest,omitempty"`
	Guidance                   string `json:"guidance"`
	CorrelationID              string `json:"correlation_id"`
}

type productRoundtableSkipSeatParams struct {
	SchemaVersion              int    `json:"schema_version"`
	SessionID                  string `json:"session_id"`
	RoundID                    string `json:"round_id"`
	InterventionID             string `json:"intervention_id"`
	ModeratorSeat              string `json:"moderator_seat"`
	SeatID                     string `json:"seat_id"`
	ExpectedMembershipRevision int    `json:"expected_membership_revision,omitempty"`
	ExpectedSeatBindingDigest  string `json:"expected_seat_binding_digest,omitempty"`
	CorrelationID              string `json:"correlation_id"`
}

type productRoundtableReplaceSeatParams struct {
	SchemaVersion  int                           `json:"schema_version"`
	SessionID      string                        `json:"session_id"`
	RoundID        string                        `json:"round_id"`
	InterventionID string                        `json:"intervention_id"`
	ModeratorSeat  string                        `json:"moderator_seat"`
	SeatID         string                        `json:"seat_id"`
	DisplayName    string                        `json:"display_name"`
	Selection      roundtable.SeatBindingRequest `json:"selection"`
	CorrelationID  string                        `json:"correlation_id"`
}

type productRoundtableExportParams struct {
	SchemaVersion int    `json:"schema_version"`
	SessionID     string `json:"session_id"`
	CorrelationID string `json:"correlation_id"`
}

type productRoundtableImportParams struct {
	SchemaVersion int    `json:"schema_version"`
	Document      []byte `json:"document"`
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
		"roundtable_pause_round", "roundtable_steer_seat", "roundtable_retry_seat",
		"roundtable_skip_seat", "roundtable_replace_seat",
		"roundtable_export", "roundtable_import",
		"roundtable_conclude", "roundtable_snapshot":
		return true
	default:
		return false
	}
}

func productRoundtableServiceError(err error) (response localipc.Response) {
	defer func() {
		response = productRoundtableResponseStage(response, "daemon_admission")
	}()
	if runtimeResponse, ok := productAgentRuntimeServiceError(err); ok {
		return runtimeResponse
	}
	switch {
	case errors.Is(err, errProductInvalidAgentInput),
		errors.Is(err, errProductActiveAttemptNotFound),
		errors.Is(err, errProductAgentInputUnsupported),
		errors.Is(err, errProductAgentInputConflict):
		return productAgentInputServiceError(err)
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
	case errors.Is(err, roundtable.ErrRoundtableAgentSeatCount):
		return productErrorResponse("seat_count_invalid", err)
	case errors.Is(err, roundtable.ErrRoundtableConflict):
		return productErrorResponse("conflict", err)
	case errors.Is(err, roundtable.ErrInvalidRoundtableSeatBinding):
		return productErrorResponse("binding_invalid", err)
	case errors.Is(err, roundtable.ErrInvalidRoundtableSeatAttempt),
		errors.Is(err, roundtable.ErrRoundtableSeatAttemptConflict):
		return productErrorResponse("attempt_invalid", err)
	case errors.Is(err, roundtable.ErrInvalidRoundtableIntervention):
		return productErrorResponse("intervention_invalid", err)
	case errors.Is(err, roundtable.ErrRoundtableInterventionConflict):
		return productErrorResponse("intervention_conflict", err)
	case errors.Is(err, roundtable.ErrRoundtableInterventionRequired):
		return productErrorResponse("intervention_required", err)
	case errors.Is(err, roundtable.ErrInvalidExportContract),
		errors.Is(err, roundtable.ErrExportContractExpired),
		errors.Is(err, roundtable.ErrExportContractNotYet),
		errors.Is(err, roundtable.ErrExportContractDigest):
		return productErrorResponse("export_invalid", err)
	case errors.Is(err, roundtable.ErrInvalidRoundtableSession),
		errors.Is(err, roundtable.ErrInvalidRoundtableSeat),
		errors.Is(err, roundtable.ErrInvalidRoundtableMessage):
		return productErrorResponse("invalid_request", err)
	default:
		return productServiceError(err)
	}
}

func productRoundtableInputError(cause error) localipc.Response {
	return productRoundtableResponseStage(
		productErrorResponse("invalid_request", cause), "input_admission",
	)
}

func productRoundtableResponseStage(
	response localipc.Response,
	stage string,
) localipc.Response {
	if response.Error != nil && response.Error.Stage == "" {
		response.Error.Stage = stage
	}
	return response
}
