package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"loom-pi-rebuild/internal/app"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/projection"
	bridgev1 "loom-pi-rebuild/protocol/bridge/v1"
)

const (
	timelineSchemaVersion = 1
	maxCursorBytes        = 32 << 10
	maxDeliveryBytes      = 8 << 10
	maxTentativeDelta     = 2048
	maxSubscribers        = 8
	maxSubscriberItems    = 64
	maxSubscriberBytes    = 64 << 10
)

var (
	ErrInvalidTimelineRequest = errors.New("invalid timeline request")
	ErrTeamTimelineNotFound   = errors.New("Team timeline not found")
	ErrInvalidTimelineCursor  = errors.New("invalid timeline cursor")
	ErrTimelineCursorConflict = errors.New("timeline cursor conflict")
	ErrTimelineScopeOverflow  = errors.New("timeline scope overflow")
	ErrTimelinePageOverflow   = errors.New("timeline page overflow")
	ErrInvalidDeliveryRecord  = errors.New("invalid delivery record")
	ErrInvalidNodeOutput      = errors.New("invalid node output")
	ErrTooManySubscribers     = errors.New("too many subscribers")
	ErrStreamGap              = errors.New("stream gap")
)

type ViewSource interface {
	Rebuild(context.Context) error
	GlobalReadView() projection.GlobalReadView
}

type TeamExecutionStreamConfig struct {
	TeamInstanceID string
	Journal        *journal.Store
	Projection     ViewSource
	Now            func() time.Time
}

type TeamExecutionStream struct {
	teamInstanceID string
	journal        *journal.Store
	projection     ViewSource
	now            func() time.Time

	mu             sync.Mutex
	nextSubscriber uint64
	subscribers    map[uint64]*Subscription
}

func NewTeamExecutionStream(
	config TeamExecutionStreamConfig,
) (*TeamExecutionStream, error) {
	if !validTimelineID(config.TeamInstanceID) ||
		config.Journal == nil ||
		config.Projection == nil ||
		config.Now == nil {
		return nil, ErrInvalidTimelineRequest
	}
	return &TeamExecutionStream{
		teamInstanceID: config.TeamInstanceID,
		journal:        config.Journal,
		projection:     config.Projection,
		now:            config.Now,
		subscribers:    make(map[uint64]*Subscription),
	}, nil
}

type timelineCursor struct {
	SchemaVersion  int                  `json:"schema_version"`
	TeamInstanceID string               `json:"team_instance_id"`
	ScopeDigest    string               `json:"scope_digest"`
	ViewVersion    string               `json:"view_version"`
	Heads          []journal.StreamHead `json:"-"`
}

type timelineCursorWire struct {
	SchemaVersion  int              `json:"schema_version"`
	TeamInstanceID string           `json:"team_instance_id"`
	ScopeDigest    string           `json:"scope_digest"`
	ViewVersion    string           `json:"view_version"`
	Heads          []cursorHeadWire `json:"heads"`
}

type cursorHeadWire struct {
	StreamID string `json:"stream_id"`
	Sequence int64  `json:"sequence"`
	EventID  string `json:"event_id"`
}

func encodeTimelineCursor(cursor timelineCursor) (string, error) {
	normalized, err := normalizeTimelineHeads(cursor.Heads)
	if err != nil ||
		cursor.SchemaVersion != timelineSchemaVersion ||
		!validTimelineID(cursor.TeamInstanceID) ||
		!validDigest(cursor.ScopeDigest) ||
		!validDigest(cursor.ViewVersion) ||
		cursor.ScopeDigest != timelineScopeDigest(normalized) {
		return "", ErrInvalidTimelineCursor
	}
	wire := timelineCursorWire{
		SchemaVersion:  cursor.SchemaVersion,
		TeamInstanceID: cursor.TeamInstanceID,
		ScopeDigest:    cursor.ScopeDigest,
		ViewVersion:    cursor.ViewVersion,
		Heads:          make([]cursorHeadWire, len(normalized)),
	}
	for index, head := range normalized {
		wire.Heads[index] = cursorHeadWire{
			StreamID: head.StreamID,
			Sequence: head.Sequence,
			EventID:  head.EventID,
		}
	}
	data, err := marshalCompact(wire)
	if err != nil {
		return "", ErrInvalidTimelineCursor
	}
	encoded := base64.RawURLEncoding.EncodeToString(data)
	if len(encoded) > maxCursorBytes {
		return "", ErrInvalidTimelineCursor
	}
	return encoded, nil
}

func decodeTimelineCursor(encoded, teamInstanceID string) (timelineCursor, error) {
	if encoded == "" || len(encoded) > maxCursorBytes ||
		!validTimelineID(teamInstanceID) ||
		strings.Contains(encoded, "=") {
		return timelineCursor{}, ErrInvalidTimelineCursor
	}
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(data) == 0 || len(data) > maxCursorBytes ||
		base64.RawURLEncoding.EncodeToString(data) != encoded {
		return timelineCursor{}, ErrInvalidTimelineCursor
	}
	var wire timelineCursorWire
	if err := decodeExactJSON(data, &wire); err != nil {
		return timelineCursor{}, ErrInvalidTimelineCursor
	}
	heads := make([]journal.StreamHead, len(wire.Heads))
	for index, head := range wire.Heads {
		heads[index] = journal.StreamHead{
			StreamID: head.StreamID,
			Sequence: head.Sequence,
			EventID:  head.EventID,
		}
	}
	normalized, err := normalizeTimelineHeads(heads)
	if err != nil ||
		wire.SchemaVersion != timelineSchemaVersion ||
		wire.TeamInstanceID != teamInstanceID ||
		!validDigest(wire.ScopeDigest) ||
		!validDigest(wire.ViewVersion) ||
		wire.ScopeDigest != timelineScopeDigest(normalized) {
		return timelineCursor{}, ErrInvalidTimelineCursor
	}
	reencoded, err := encodeTimelineCursor(timelineCursor{
		SchemaVersion:  wire.SchemaVersion,
		TeamInstanceID: wire.TeamInstanceID,
		ScopeDigest:    wire.ScopeDigest,
		ViewVersion:    wire.ViewVersion,
		Heads:          normalized,
	})
	if err != nil || reencoded != encoded {
		return timelineCursor{}, ErrInvalidTimelineCursor
	}
	return timelineCursor{
		SchemaVersion:  wire.SchemaVersion,
		TeamInstanceID: wire.TeamInstanceID,
		ScopeDigest:    wire.ScopeDigest,
		ViewVersion:    wire.ViewVersion,
		Heads:          normalized,
	}, nil
}

func timelineScopeDigest(heads []journal.StreamHead) string {
	hasher := sha256.New()
	writeDigestField(hasher, "loom.team-stream-scope.v1")
	for _, head := range heads {
		writeDigestField(hasher, head.StreamID)
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func timelineCursorDigest(encoded string) string {
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type CostObservation struct {
	observed         bool
	amountMicrounits *int64
	currency         string
}

type costWire struct {
	Observed         bool   `json:"observed"`
	AmountMicrounits *int64 `json:"amount_microunits"`
	Currency         string `json:"currency"`
}

func (cost CostObservation) MarshalJSON() ([]byte, error) {
	return marshalCompact(costWire{
		Observed:         cost.observed,
		AmountMicrounits: cost.amountMicrounits,
		Currency:         cost.currency,
	})
}

type DeliveryPayload struct {
	status         string
	reasonCode     string
	action         string
	warningCode    string
	retryAt        string
	textDelta      string
	evidenceDigest string
	cost           CostObservation
}

type deliveryPayloadWire struct {
	Status         string          `json:"status"`
	ReasonCode     string          `json:"reason_code"`
	Action         string          `json:"action"`
	WarningCode    string          `json:"warning_code"`
	RetryAt        string          `json:"retry_at"`
	TextDelta      string          `json:"text_delta"`
	EvidenceDigest string          `json:"evidence_digest"`
	Cost           CostObservation `json:"cost"`
}

type DeliveryRecord struct {
	schemaVersion   int
	deliveryID      string
	kind            string
	authority       string
	teamInstanceID  string
	logicalNodeID   string
	attemptNumber   int
	sourceStreamID  string
	sourceSequence  int64
	sourceEventID   string
	occurredAt      time.Time
	cursor          string
	payload         DeliveryPayload
	runID           string
	claimGeneration int64
}

type deliveryRecordWire struct {
	SchemaVersion  int                 `json:"schema_version"`
	DeliveryID     string              `json:"delivery_id"`
	Kind           string              `json:"kind"`
	Authority      string              `json:"authority"`
	TeamInstanceID string              `json:"team_instance_id"`
	LogicalNodeID  string              `json:"logical_node_id"`
	AttemptNumber  int                 `json:"attempt_number"`
	SourceStreamID string              `json:"source_stream_id"`
	SourceSequence int64               `json:"source_sequence"`
	SourceEventID  string              `json:"source_event_id"`
	OccurredAt     string              `json:"occurred_at"`
	Cursor         string              `json:"cursor"`
	Payload        deliveryPayloadWire `json:"payload"`
}

func (record DeliveryRecord) MarshalJSON() ([]byte, error) {
	wire := deliveryRecordWire{
		SchemaVersion:  record.schemaVersion,
		DeliveryID:     record.deliveryID,
		Kind:           record.kind,
		Authority:      record.authority,
		TeamInstanceID: record.teamInstanceID,
		LogicalNodeID:  record.logicalNodeID,
		AttemptNumber:  record.attemptNumber,
		SourceStreamID: record.sourceStreamID,
		SourceSequence: record.sourceSequence,
		SourceEventID:  record.sourceEventID,
		OccurredAt:     canonicalTime(record.occurredAt),
		Cursor:         record.cursor,
		Payload: deliveryPayloadWire{
			Status:         record.payload.status,
			ReasonCode:     record.payload.reasonCode,
			Action:         record.payload.action,
			WarningCode:    record.payload.warningCode,
			RetryAt:        record.payload.retryAt,
			TextDelta:      record.payload.textDelta,
			EvidenceDigest: record.payload.evidenceDigest,
			Cost:           record.payload.cost,
		},
	}
	data, err := marshalCompact(wire)
	if err != nil || len(data) > maxDeliveryBytes {
		return nil, ErrInvalidDeliveryRecord
	}
	return data, nil
}

type StreamGap struct {
	schemaVersion        int
	deliveryID           string
	teamInstanceID       string
	reason               string
	previousCursorDigest string
	currentViewVersion   string
	artifactAvailable    bool
	artifactDigest       string
	recoverable          bool
	occurredAt           time.Time
}

type streamGapWire struct {
	SchemaVersion        int    `json:"schema_version"`
	DeliveryID           string `json:"delivery_id"`
	Kind                 string `json:"kind"`
	TeamInstanceID       string `json:"team_instance_id"`
	Reason               string `json:"reason"`
	PreviousCursorDigest string `json:"previous_cursor_digest"`
	CurrentViewVersion   string `json:"current_view_version"`
	ArtifactAvailable    bool   `json:"artifact_available"`
	ArtifactDigest       string `json:"artifact_digest"`
	Recoverable          bool   `json:"recoverable"`
	OccurredAt           string `json:"occurred_at"`
}

func (gap StreamGap) MarshalJSON() ([]byte, error) {
	return marshalCompact(streamGapWire{
		SchemaVersion:        gap.schemaVersion,
		DeliveryID:           gap.deliveryID,
		Kind:                 "stream_gap",
		TeamInstanceID:       gap.teamInstanceID,
		Reason:               gap.reason,
		PreviousCursorDigest: gap.previousCursorDigest,
		CurrentViewVersion:   gap.currentViewVersion,
		ArtifactAvailable:    gap.artifactAvailable,
		ArtifactDigest:       gap.artifactDigest,
		Recoverable:          gap.recoverable,
		OccurredAt:           canonicalTime(gap.occurredAt),
	})
}

func (gap StreamGap) DeliveryID() string { return gap.deliveryID }
func (gap StreamGap) Reason() string     { return gap.reason }
func (gap StreamGap) Recoverable() bool  { return gap.recoverable }

type streamGapInput struct {
	TeamInstanceID       string
	Reason               string
	PreviousCursorDigest string
	CurrentViewVersion   string
	ArtifactDigest       string
	OccurredAt           time.Time
}

func newStreamGap(input streamGapInput) (StreamGap, error) {
	if !validTimelineID(input.TeamInstanceID) ||
		!validGapReason(input.Reason) ||
		input.PreviousCursorDigest != "" &&
			!validDigest(input.PreviousCursorDigest) ||
		input.CurrentViewVersion != "" &&
			!validDigest(input.CurrentViewVersion) ||
		input.ArtifactDigest != "" && !validDigest(input.ArtifactDigest) ||
		input.OccurredAt.IsZero() ||
		input.OccurredAt.Location() != time.UTC {
		return StreamGap{}, ErrInvalidDeliveryRecord
	}
	id := digestFields(
		"loom.delivery.gap.v1",
		input.TeamInstanceID,
		input.Reason,
		input.PreviousCursorDigest,
		input.CurrentViewVersion,
		input.ArtifactDigest,
		canonicalTime(input.OccurredAt),
	)
	return StreamGap{
		schemaVersion:        timelineSchemaVersion,
		deliveryID:           id,
		teamInstanceID:       input.TeamInstanceID,
		reason:               input.Reason,
		previousCursorDigest: input.PreviousCursorDigest,
		currentViewVersion:   input.CurrentViewVersion,
		artifactAvailable:    input.ArtifactDigest != "",
		artifactDigest:       input.ArtifactDigest,
		recoverable:          true,
		occurredAt:           input.OccurredAt,
	}, nil
}

type TeamBoard struct {
	schemaVersion  int
	teamInstanceID string
	planDigest     string
	status         string
	viewVersion    string
	nodes          []NodeBoardRow
	cost           CostObservation
}

type NodeBoardRow struct {
	LogicalNodeID       string `json:"logical_node_id"`
	Status              string `json:"status"`
	DependencySatisfied bool   `json:"dependency_satisfied"`
	CurrentAttempt      int    `json:"current_attempt"`
	WorkItemID          string `json:"work_item_id"`
	RunID               string `json:"run_id"`
	RuntimeInstanceID   string `json:"runtime_instance_id"`
	AgentInstanceID     string `json:"agent_instance_id"`
	VerificationStatus  string `json:"verification_status"`
	RecoveryAction      string `json:"recovery_action"`
	RetryAt             string `json:"retry_at"`
}

type teamBoardWire struct {
	SchemaVersion  int             `json:"schema_version"`
	TeamInstanceID string          `json:"team_instance_id"`
	PlanDigest     string          `json:"plan_digest"`
	Status         string          `json:"status"`
	ViewVersion    string          `json:"view_version"`
	Nodes          []NodeBoardRow  `json:"nodes"`
	Cost           CostObservation `json:"cost"`
}

func (board TeamBoard) MarshalJSON() ([]byte, error) {
	nodes := append([]NodeBoardRow(nil), board.nodes...)
	if nodes == nil {
		nodes = []NodeBoardRow{}
	}
	return marshalCompact(teamBoardWire{
		SchemaVersion:  board.schemaVersion,
		TeamInstanceID: board.teamInstanceID,
		PlanDigest:     board.planDigest,
		Status:         board.status,
		ViewVersion:    board.viewVersion,
		Nodes:          nodes,
		Cost:           board.cost,
	})
}

type AttentionItem struct {
	SchemaVersion     int    `json:"schema_version"`
	AttentionID       string `json:"attention_id"`
	Kind              string `json:"kind"`
	Severity          string `json:"severity"`
	TeamInstanceID    string `json:"team_instance_id"`
	LogicalNodeID     string `json:"logical_node_id"`
	WorkItemID        string `json:"work_item_id"`
	ApprovalRequestID string `json:"approval_request_id"`
	RuntimeInstanceID string `json:"runtime_instance_id"`
	Status            string `json:"status"`
	OccurredAt        string `json:"occurred_at"`
	ActionRequired    string `json:"action_required"`
}

type TimelinePage struct {
	teamInstanceID string
	viewVersion    string
	nextCursor     string
	hasMore        bool
	gap            *StreamGap
	records        []DeliveryRecord
	board          TeamBoard
	attention      []AttentionItem
}

type timelinePageWire struct {
	SchemaVersion  int              `json:"schema_version"`
	TeamInstanceID string           `json:"team_instance_id"`
	ViewVersion    string           `json:"view_version"`
	NextCursor     string           `json:"next_cursor"`
	HasMore        bool             `json:"has_more"`
	Gap            *StreamGap       `json:"gap"`
	Records        []DeliveryRecord `json:"records"`
	Board          TeamBoard        `json:"board"`
	Attention      []AttentionItem  `json:"attention"`
}

func (page TimelinePage) MarshalJSON() ([]byte, error) {
	return marshalCompact(page.wire())
}

func (page TimelinePage) wire() timelinePageWire {
	records := append([]DeliveryRecord(nil), page.records...)
	attention := append([]AttentionItem(nil), page.attention...)
	if records == nil {
		records = []DeliveryRecord{}
	}
	if attention == nil {
		attention = []AttentionItem{}
	}
	var gap *StreamGap
	if page.gap != nil {
		copied := *page.gap
		gap = &copied
	}
	return timelinePageWire{
		SchemaVersion:  timelineSchemaVersion,
		TeamInstanceID: page.teamInstanceID,
		ViewVersion:    page.viewVersion,
		NextCursor:     page.nextCursor,
		HasMore:        page.hasMore,
		Gap:            gap,
		Records:        records,
		Board:          cloneBoard(page.board),
		Attention:      attention,
	}
}

func (page TimelinePage) TeamInstanceID() string { return page.teamInstanceID }
func (page TimelinePage) ViewVersion() string    { return page.viewVersion }
func (page TimelinePage) NextCursor() string     { return page.nextCursor }
func (page TimelinePage) HasMore() bool          { return page.hasMore }
func (page TimelinePage) Gap() (StreamGap, bool) {
	if page.gap == nil {
		return StreamGap{}, false
	}
	return *page.gap, true
}
func (page TimelinePage) Records() []DeliveryRecord {
	return append([]DeliveryRecord(nil), page.records...)
}
func (page TimelinePage) Board() TeamBoard { return cloneBoard(page.board) }
func (page TimelinePage) Attention() []AttentionItem {
	return append([]AttentionItem(nil), page.attention...)
}

func newTimelineGapPage(teamInstanceID string, gap StreamGap) TimelinePage {
	return TimelinePage{
		teamInstanceID: teamInstanceID,
		viewVersion:    gap.currentViewVersion,
		gap:            &gap,
		records:        []DeliveryRecord{},
		board: TeamBoard{
			schemaVersion:  timelineSchemaVersion,
			teamInstanceID: teamInstanceID,
			viewVersion:    gap.currentViewVersion,
			nodes:          []NodeBoardRow{},
		},
		attention: []AttentionItem{},
	}
}

type TimelineGapError struct {
	page   TimelinePage
	causes []error
}

func newTimelineGapError(
	page TimelinePage,
	specific error,
) *TimelineGapError {
	return &TimelineGapError{
		page:   cloneTimelinePage(page),
		causes: []error{ErrStreamGap, specific},
	}
}

func (*TimelineGapError) Error() string { return "stream gap" }
func (err *TimelineGapError) Unwrap() []error {
	return append([]error(nil), err.causes...)
}
func (err *TimelineGapError) Page() TimelinePage {
	return cloneTimelinePage(err.page)
}

func (stream *TeamExecutionStream) ReadPage(
	ctx context.Context,
	cursor string,
	limit int,
) (TimelinePage, error) {
	if err := ctx.Err(); err != nil {
		return TimelinePage{}, err
	}
	if limit < 1 || limit > journal.MaxReadPageEvents {
		return stream.requestGap(
			ctx,
			cursor,
			"page_overflow",
			ErrTimelinePageOverflow,
		)
	}
	var decoded timelineCursor
	if cursor != "" {
		var err error
		decoded, err = decodeTimelineCursor(cursor, stream.teamInstanceID)
		if err != nil {
			return stream.requestGap(
				ctx,
				cursor,
				"invalid_cursor",
				ErrInvalidTimelineCursor,
			)
		}
	}
	if err := stream.projection.Rebuild(ctx); err != nil {
		return TimelinePage{}, err
	}
	viewA := stream.projection.GlobalReadView()
	scopeA, err := deriveRelatedScope(viewA, stream.teamInstanceID)
	if err != nil {
		return TimelinePage{}, err
	}
	heads, err := unionCursorHeads(decoded.Heads, scopeA)
	if err != nil {
		return stream.gapFromView(
			cursor,
			"scope_overflow",
			ErrTimelineScopeOverflow,
			viewA,
		)
	}
	if cursor == "" {
		decoded = timelineCursor{
			SchemaVersion:  timelineSchemaVersion,
			TeamInstanceID: stream.teamInstanceID,
			ViewVersion:    viewA.Version(),
			Heads:          heads,
		}
		decoded.ScopeDigest = timelineScopeDigest(heads)
	}
	journalPage, err := stream.journal.ReadPageAfterHeads(ctx, heads, limit)
	if err != nil {
		switch {
		case errors.Is(err, journal.ErrInvalidStreamCursor):
			return stream.gapFromView(cursor, "invalid_cursor", ErrInvalidTimelineCursor, viewA)
		case errors.Is(err, journal.ErrStreamCursorConflict),
			errors.Is(err, journal.ErrStreamSequenceGap):
			return stream.gapFromView(cursor, "cursor_conflict", ErrTimelineCursorConflict, viewA)
		case errors.Is(err, journal.ErrStreamPageLimit):
			return stream.gapFromView(cursor, "page_overflow", ErrTimelinePageOverflow, viewA)
		default:
			return TimelinePage{}, err
		}
	}
	if err := stream.projection.Rebuild(ctx); err != nil {
		return TimelinePage{}, err
	}
	viewB := stream.projection.GlobalReadView()
	scopeB, err := deriveRelatedScope(viewB, stream.teamInstanceID)
	if err != nil {
		return TimelinePage{}, err
	}
	nextHeads, err := unionCursorHeads(journalPage.Heads(), scopeB)
	if err != nil {
		return stream.gapFromView(cursor, "scope_overflow", ErrTimelineScopeOverflow, viewB)
	}
	nextCursorState := timelineCursor{
		SchemaVersion:  timelineSchemaVersion,
		TeamInstanceID: stream.teamInstanceID,
		ViewVersion:    viewB.Version(),
		Heads:          nextHeads,
	}
	nextCursorState.ScopeDigest = timelineScopeDigest(nextHeads)
	nextCursor, err := encodeTimelineCursor(nextCursorState)
	if err != nil {
		return TimelinePage{}, err
	}
	records, err := mapAuthoritativeRecords(
		stream.teamInstanceID,
		viewB,
		heads,
		journalPage.Events(),
	)
	if err != nil {
		return TimelinePage{}, err
	}
	board, attention := deriveBoardAndAttention(
		viewB,
		stream.teamInstanceID,
	)
	return TimelinePage{
		teamInstanceID: stream.teamInstanceID,
		viewVersion:    viewB.Version(),
		nextCursor:     nextCursor,
		hasMore:        journalPage.HasMore(),
		records:        records,
		board:          board,
		attention:      attention,
	}, nil
}

func (stream *TeamExecutionStream) requestGap(
	ctx context.Context,
	cursor,
	reason string,
	specific error,
) (TimelinePage, error) {
	if err := ctx.Err(); err != nil {
		return TimelinePage{}, err
	}
	_ = stream.projection.Rebuild(ctx)
	view := stream.projection.GlobalReadView()
	return stream.gapFromView(cursor, reason, specific, view)
}

func (stream *TeamExecutionStream) gapFromView(
	cursor,
	reason string,
	specific error,
	view projection.GlobalReadView,
) (TimelinePage, error) {
	now := stream.now()
	if now.IsZero() || now.Location() != time.UTC {
		return TimelinePage{}, ErrInvalidTimelineRequest
	}
	gap, err := newStreamGap(streamGapInput{
		TeamInstanceID:       stream.teamInstanceID,
		Reason:               reason,
		PreviousCursorDigest: timelineCursorDigest(cursor),
		CurrentViewVersion:   view.Version(),
		ArtifactDigest:       latestTeamArtifactDigest(view, stream.teamInstanceID),
		OccurredAt:           now,
	})
	if err != nil {
		return TimelinePage{}, err
	}
	page := newTimelineGapPage(stream.teamInstanceID, gap)
	if _, ok := view.TeamExecution(stream.teamInstanceID); ok {
		page.board, page.attention = deriveBoardAndAttention(
			view,
			stream.teamInstanceID,
		)
	}
	page.attention = append(page.attention, attentionFromGap(gap))
	sortAttention(page.attention)
	gapErr := newTimelineGapError(page, specific)
	return page, gapErr
}

func (stream *TeamExecutionStream) Subscribe(
	ctx context.Context,
	cursor string,
) (*Subscription, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var decoded timelineCursor
	if cursor != "" {
		var err error
		decoded, err = decodeTimelineCursor(cursor, stream.teamInstanceID)
		if err != nil {
			return nil, ErrInvalidTimelineCursor
		}
	}
	if err := stream.projection.Rebuild(ctx); err != nil {
		return nil, err
	}
	view := stream.projection.GlobalReadView()
	scope, err := deriveRelatedScope(view, stream.teamInstanceID)
	if err != nil {
		return nil, err
	}
	heads, err := unionCursorHeads(decoded.Heads, scope)
	if err != nil {
		return nil, ErrTimelineScopeOverflow
	}
	if _, err := stream.journal.ReadPageAfterHeads(ctx, heads, 1); err != nil {
		switch {
		case errors.Is(err, journal.ErrInvalidStreamCursor):
			return nil, ErrInvalidTimelineCursor
		case errors.Is(err, journal.ErrStreamCursorConflict),
			errors.Is(err, journal.ErrStreamSequenceGap):
			return nil, ErrTimelineCursorConflict
		default:
			return nil, err
		}
	}
	stream.mu.Lock()
	defer stream.mu.Unlock()
	if len(stream.subscribers) >= maxSubscribers {
		return nil, ErrTooManySubscribers
	}
	stream.nextSubscriber++
	subscription := &Subscription{
		id:     stream.nextSubscriber,
		owner:  stream,
		cursor: cursor,
		notify: make(chan struct{}, 1),
		queue:  make([]StreamItem, 0),
	}
	stream.subscribers[subscription.id] = subscription
	return subscription, nil
}

type StreamItem struct {
	delivery *DeliveryRecord
	gap      *StreamGap
}

func (item StreamItem) Delivery() (DeliveryRecord, bool) {
	if item.delivery == nil {
		return DeliveryRecord{}, false
	}
	return *item.delivery, true
}
func (item StreamItem) Gap() (StreamGap, bool) {
	if item.gap == nil {
		return StreamGap{}, false
	}
	return *item.gap, true
}

type Subscription struct {
	id     uint64
	owner  *TeamExecutionStream
	cursor string
	notify chan struct{}

	mu      sync.Mutex
	queue   []StreamItem
	bytes   int
	pending *StreamGap
	closed  bool
}

func (subscription *Subscription) Next(
	ctx context.Context,
) (StreamItem, error) {
	for {
		subscription.mu.Lock()
		if subscription.closed {
			subscription.mu.Unlock()
			return StreamItem{}, context.Canceled
		}
		if subscription.pending != nil {
			gap := *subscription.pending
			subscription.pending = nil
			subscription.mu.Unlock()
			return StreamItem{gap: &gap}, nil
		}
		if len(subscription.queue) > 0 {
			item := subscription.queue[0]
			subscription.queue = append([]StreamItem(nil), subscription.queue[1:]...)
			if delivery, ok := item.Delivery(); ok {
				data, _ := json.Marshal(delivery)
				subscription.bytes -= len(data)
			}
			subscription.mu.Unlock()
			return item, nil
		}
		subscription.mu.Unlock()
		select {
		case <-ctx.Done():
			return StreamItem{}, ctx.Err()
		case <-subscription.notify:
		}
	}
}

func (subscription *Subscription) Close() error {
	subscription.mu.Lock()
	if subscription.closed {
		subscription.mu.Unlock()
		return nil
	}
	subscription.closed = true
	subscription.queue = nil
	subscription.bytes = 0
	subscription.mu.Unlock()
	subscription.owner.mu.Lock()
	delete(subscription.owner.subscribers, subscription.id)
	subscription.owner.mu.Unlock()
	select {
	case subscription.notify <- struct{}{}:
	default:
	}
	return nil
}

func (stream *TeamExecutionStream) ObserveNodeOutput(
	_ context.Context,
	output app.NodeOutput,
) error {
	view := stream.projection.GlobalReadView()
	execution, ok := view.TeamExecution(stream.teamInstanceID)
	if !ok || !output.Tentative() ||
		!validTimelineID(output.LogicalNodeID()) ||
		output.AttemptNumber() <= 0 {
		return ErrInvalidNodeOutput
	}
	authorized := output.AuthorizedFrame()
	frame := authorized.Frame()
	binding := authorized.Binding()
	if !authorized.Tentative() {
		return ErrInvalidNodeOutput
	}
	if frame.Type() != bridgev1.MessageEvent {
		return nil
	}
	if frame.Sequence() <= 0 ||
		!validTimelineID(frame.MessageID()) {
		return ErrInvalidNodeOutput
	}
	attempt, ok := findProjectedAttempt(
		execution,
		output.LogicalNodeID(),
		output.AttemptNumber(),
	)
	if !ok ||
		attempt.WorkItemID != binding.WorkItemID ||
		attempt.RunID != binding.RunID ||
		attempt.ClaimGeneration != binding.ClaimGeneration ||
		attempt.RuntimeInstanceID != binding.RuntimeInstanceID ||
		attempt.AgentInstanceID != binding.SenderAgentInstanceID {
		return ErrInvalidNodeOutput
	}
	delta, err := decodeTentativeDelta(frame.Payload())
	if err != nil {
		return ErrInvalidNodeOutput
	}
	record := DeliveryRecord{
		schemaVersion: timelineSchemaVersion,
		deliveryID: digestFields(
			"loom.delivery.tentative.v1",
			stream.teamInstanceID,
			output.LogicalNodeID(),
			strconv.Itoa(output.AttemptNumber()),
			binding.WorkItemID,
			binding.RunID,
			strconv.FormatInt(binding.ClaimGeneration, 10),
			frame.MessageID(),
			strconv.FormatInt(frame.Sequence(), 10),
		),
		kind:            "node_output_delta",
		authority:       "tentative",
		teamInstanceID:  stream.teamInstanceID,
		logicalNodeID:   output.LogicalNodeID(),
		attemptNumber:   output.AttemptNumber(),
		sourceSequence:  frame.Sequence(),
		sourceEventID:   frame.MessageID(),
		occurredAt:      frame.EmittedAt(),
		runID:           binding.RunID,
		claimGeneration: binding.ClaimGeneration,
		payload: DeliveryPayload{
			textDelta: delta,
		},
	}
	stream.mu.Lock()
	subscribers := make([]*Subscription, 0, len(stream.subscribers))
	for _, subscription := range stream.subscribers {
		subscribers = append(subscribers, subscription)
	}
	stream.mu.Unlock()
	for _, subscription := range subscribers {
		record.cursor = subscription.cursor
		subscription.enqueue(record)
	}
	return nil
}

func decodeTentativeDelta(payloadJSON []byte) (string, error) {
	var payload struct {
		Delta *string `json:"delta"`
	}
	if err := decodeExactJSON(payloadJSON, &payload); err != nil ||
		payload.Delta == nil ||
		!validTentativeDelta(*payload.Delta) {
		return "", ErrInvalidNodeOutput
	}
	return *payload.Delta, nil
}

func (subscription *Subscription) enqueue(record DeliveryRecord) {
	data, err := json.Marshal(record)
	if err != nil {
		return
	}
	subscription.mu.Lock()
	defer subscription.mu.Unlock()
	if subscription.closed {
		return
	}
	if len(subscription.queue) > 0 {
		last := subscription.queue[len(subscription.queue)-1].delivery
		if last != nil &&
			last.teamInstanceID == record.teamInstanceID &&
			last.logicalNodeID == record.logicalNodeID &&
			last.attemptNumber == record.attemptNumber &&
			last.runID == record.runID &&
			last.claimGeneration == record.claimGeneration &&
			len(last.payload.textDelta)+len(record.payload.textDelta) <= maxTentativeDelta {
			previous, previousErr := json.Marshal(*last)
			candidate := *last
			candidate.payload.textDelta += record.payload.textDelta
			candidate.sourceSequence = record.sourceSequence
			candidate.sourceEventID = record.sourceEventID
			updated, marshalErr := json.Marshal(candidate)
			if marshalErr == nil &&
				previousErr == nil &&
				subscription.bytes-len(previous)+len(updated) <= maxSubscriberBytes {
				*last = candidate
				subscription.bytes = subscription.bytes - len(previous) + len(updated)
				return
			}
		}
	}
	if len(subscription.queue) >= maxSubscriberItems ||
		subscription.bytes+len(data) > maxSubscriberBytes {
		gap, gapErr := newStreamGap(streamGapInput{
			TeamInstanceID:       record.teamInstanceID,
			Reason:               "tentative_overflow",
			PreviousCursorDigest: timelineCursorDigest(subscription.cursor),
			OccurredAt:           record.occurredAt,
		})
		if gapErr == nil {
			subscription.pending = &gap
		}
		select {
		case subscription.notify <- struct{}{}:
		default:
		}
		return
	}
	copied := record
	subscription.queue = append(
		subscription.queue,
		StreamItem{delivery: &copied},
	)
	subscription.bytes += len(data)
	select {
	case subscription.notify <- struct{}{}:
	default:
	}
}

func deriveRelatedScope(
	view projection.GlobalReadView,
	teamInstanceID string,
) ([]string, error) {
	if _, ok := view.TeamTimelineAnchor(teamInstanceID); !ok {
		return nil, ErrTeamTimelineNotFound
	}
	execution, ok := view.TeamExecution(teamInstanceID)
	if !ok {
		return nil, ErrTeamTimelineNotFound
	}
	streams := map[string]struct{}{
		"team-execution/" + teamInstanceID: {},
	}
	runIDs := make(map[string]struct{})
	for _, node := range execution.Nodes {
		for _, attempt := range node.Attempts {
			if attempt.WorkItemID != "" {
				logicalNodeID, attemptNumber, ok := relatedWorkItemLineage(
					view,
					execution,
					teamInstanceID,
					attempt.WorkItemID,
				)
				if !ok ||
					logicalNodeID != node.LogicalNodeID ||
					attemptNumber != attempt.AttemptNumber {
					return nil, ErrInvalidDeliveryRecord
				}
				streams["work-item/"+attempt.WorkItemID] = struct{}{}
			}
			if attempt.RunID != "" {
				logicalNodeID, attemptNumber, ok := relatedRunLineage(
					view,
					execution,
					teamInstanceID,
					attempt.RunID,
				)
				if !ok ||
					logicalNodeID != node.LogicalNodeID ||
					attemptNumber != attempt.AttemptNumber {
					return nil, ErrInvalidDeliveryRecord
				}
				streams["run/"+attempt.RunID] = struct{}{}
				runIDs[attempt.RunID] = struct{}{}
			}
			if attempt.EvidenceID != "" {
				logicalNodeID, attemptNumber, ok := relatedEvidenceLineage(
					view,
					execution,
					teamInstanceID,
					attempt.EvidenceID,
				)
				if !ok ||
					logicalNodeID != node.LogicalNodeID ||
					attemptNumber != attempt.AttemptNumber {
					return nil, ErrInvalidDeliveryRecord
				}
				streams["evidence/"+attempt.EvidenceID] = struct{}{}
			}
		}
	}
	for _, workItem := range view.WorkItemsForTeam(teamInstanceID) {
		logicalNodeID, attemptNumber, ok := relatedWorkItemLineage(
			view,
			execution,
			teamInstanceID,
			workItem.ID,
		)
		if !ok ||
			logicalNodeID != workItem.LogicalNodeID ||
			attemptNumber != workItem.AttemptNumber {
			return nil, ErrInvalidDeliveryRecord
		}
		streams["work-item/"+workItem.ID] = struct{}{}
		if workItem.VerifierWorkItemID != "" {
			logicalNodeID, attemptNumber, ok = relatedWorkItemLineage(
				view,
				execution,
				teamInstanceID,
				workItem.VerifierWorkItemID,
			)
			if !ok ||
				logicalNodeID != workItem.LogicalNodeID ||
				attemptNumber != workItem.AttemptNumber {
				return nil, ErrInvalidDeliveryRecord
			}
			streams["work-item/"+workItem.VerifierWorkItemID] =
				struct{}{}
		}
		if workItem.RunID != "" {
			streams["run/"+workItem.RunID] = struct{}{}
			runIDs[workItem.RunID] = struct{}{}
		}
		for _, evidenceID := range []string{
			workItem.SourceEvidenceID,
			workItem.VerifierEvidenceID,
		} {
			if evidenceID != "" {
				logicalNodeID, attemptNumber, ok :=
					relatedEvidenceLineage(
						view,
						execution,
						teamInstanceID,
						evidenceID,
					)
				if !ok ||
					logicalNodeID != workItem.LogicalNodeID ||
					attemptNumber != workItem.AttemptNumber {
					return nil, ErrInvalidDeliveryRecord
				}
				streams["evidence/"+evidenceID] = struct{}{}
			}
		}
		if workItem.VerifierRunID != "" {
			streams["run/"+workItem.VerifierRunID] = struct{}{}
			runIDs[workItem.VerifierRunID] = struct{}{}
		}
	}
	for runID := range runIDs {
		streams["agent-grant/"+runID] = struct{}{}
		run, runFound := view.Run(runID)
		if !runFound {
			return nil, ErrInvalidDeliveryRecord
		}
		if _, _, ok := relatedRunLineage(
			view,
			execution,
			teamInstanceID,
			runID,
		); !ok {
			return nil, ErrInvalidDeliveryRecord
		}
		for _, grant := range view.AgentGrantsForRun(runID) {
			if grant.RunID != runID ||
				grant.WorkItemID != run.WorkItemID ||
				grant.RuntimeInstanceID != run.RuntimeInstanceID ||
				grant.AgentInstanceID != run.AgentInstanceID ||
				grant.ClaimGeneration <= 0 ||
				grant.ClaimGeneration > run.ClaimGeneration {
				return nil, ErrInvalidDeliveryRecord
			}
		}
	}
	for _, approval := range view.ApprovalRequestsForTeam(teamInstanceID) {
		if approval.ID != "" {
			logicalNodeID, attemptNumber, ok := relatedApprovalLineage(
				view,
				execution,
				teamInstanceID,
				approval.ID,
			)
			if !ok ||
				logicalNodeID != approval.LogicalNodeID ||
				attemptNumber != approval.AttemptNumber {
				return nil, ErrInvalidDeliveryRecord
			}
			streams["approval/"+approval.ID] = struct{}{}
		}
	}
	if len(streams) > journal.MaxCursorStreams {
		return nil, ErrTimelineScopeOverflow
	}
	result := make([]string, 0, len(streams))
	for streamID := range streams {
		result = append(result, streamID)
	}
	sort.Strings(result)
	return result, nil
}

func unionCursorHeads(
	old []journal.StreamHead,
	current []string,
) ([]journal.StreamHead, error) {
	heads := make(map[string]journal.StreamHead, len(old)+len(current))
	for _, head := range old {
		heads[head.StreamID] = head
	}
	for _, streamID := range current {
		if _, ok := heads[streamID]; !ok {
			heads[streamID] = journal.StreamHead{StreamID: streamID}
		}
	}
	if len(heads) == 0 || len(heads) > journal.MaxCursorStreams {
		return nil, ErrTimelineScopeOverflow
	}
	result := make([]journal.StreamHead, 0, len(heads))
	for _, head := range heads {
		result = append(result, head)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].StreamID < result[j].StreamID
	})
	return result, nil
}

var authoritativeKinds = map[string]string{
	"TeamExecutionPlanned":          "team_planned",
	"TeamNodeAttemptScheduled":      "node_scheduled",
	"TeamReadySetDispatched":        "ready_set_dispatched",
	"TeamNodeAttemptRebound":        "node_rebound",
	"RunStarted":                    "run_started",
	"RunTerminalCommitted":          "run_terminal",
	"WorkItemApprovalPaused":        "approval_required",
	"ApprovalRequested":             "approval_requested",
	"ApprovalDecided":               "approval_decided",
	"ApprovalExpired":               "approval_expired",
	"WorkItemApprovalResolved":      "approval_resolved",
	"TeamNodeAttemptTerminal":       "node_attempt_terminal",
	"WorkItemReadyForReview":        "ready_for_review",
	"WorkItemVerificationCommitted": "verification_recorded",
	"WorkItemDone":                  "work_item_done",
	"WorkItemRejected":              "verification_rejected",
	"TeamNodeAcceptanceCommitted":   "node_acceptance",
	"TeamNodeRecoveryRecorded":      "node_recovery",
	"EvidenceSubmitted":             "evidence_available",
	"TeamExecutionTerminal":         "team_terminal",
}

func mapAuthoritativeRecords(
	teamInstanceID string,
	view timelineLineageView,
	startHeads []journal.StreamHead,
	events []journal.Event,
) ([]DeliveryRecord, error) {
	heads, err := normalizeTimelineHeads(startHeads)
	if err != nil {
		return nil, err
	}
	headMap := make(map[string]journal.StreamHead, len(heads))
	for _, head := range heads {
		headMap[head.StreamID] = head
	}
	records := make([]DeliveryRecord, 0)
	for _, event := range events {
		head := headMap[event.StreamID]
		head.Sequence = event.Seq
		head.EventID = event.ID
		headMap[event.StreamID] = head
		cursorHeads := make([]journal.StreamHead, 0, len(headMap))
		for _, current := range headMap {
			cursorHeads = append(cursorHeads, current)
		}
		sort.Slice(cursorHeads, func(i, j int) bool {
			return cursorHeads[i].StreamID < cursorHeads[j].StreamID
		})
		cursorState := timelineCursor{
			SchemaVersion:  timelineSchemaVersion,
			TeamInstanceID: teamInstanceID,
			ViewVersion:    view.Version(),
			Heads:          cursorHeads,
		}
		cursorState.ScopeDigest = timelineScopeDigest(cursorHeads)
		cursor, err := encodeTimelineCursor(cursorState)
		if err != nil {
			return nil, err
		}
		kind, ok := authoritativeKinds[event.Type]
		if !ok {
			continue
		}
		safe, err := safeEventFields(event.PayloadJSON)
		if err != nil {
			return nil, err
		}
		logicalNodeID, attemptNumber, err := resolveDeliveryLineage(
			teamInstanceID,
			view,
			event,
			kind,
			safe,
		)
		if err != nil {
			return nil, err
		}
		record := DeliveryRecord{
			schemaVersion: timelineSchemaVersion,
			deliveryID: digestFields(
				"loom.delivery.journal.v1",
				teamInstanceID,
				kind,
				event.StreamID,
				strconv.FormatInt(event.Seq, 10),
				event.ID,
			),
			kind:           kind,
			authority:      "journal",
			teamInstanceID: teamInstanceID,
			logicalNodeID:  logicalNodeID,
			attemptNumber:  attemptNumber,
			sourceStreamID: event.StreamID,
			sourceSequence: event.Seq,
			sourceEventID:  event.ID,
			occurredAt:     event.EmittedAt,
			cursor:         cursor,
			payload: DeliveryPayload{
				status:         firstNonempty(safe["status"], safe["acceptance_decision_kind"]),
				reasonCode:     firstNonempty(safe["reason_code"], safe["reason"]),
				action:         firstNonempty(safe["action"], safe["recovery_action"]),
				warningCode:    safe["warning_code"],
				retryAt:        safe["retry_at"],
				evidenceDigest: firstNonempty(safe["evidence_digest"], safe["source_evidence_digest"]),
			},
		}
		if _, err := json.Marshal(record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}
	return records, nil
}

type timelineLineageView interface {
	Version() string
	TeamExecution(string) (projection.TeamExecution, bool)
	WorkItem(string) (projection.WorkItem, bool)
	WorkItemsForTeam(string) []projection.WorkItem
	Run(string) (projection.Run, bool)
	Evidence(string) (projection.Evidence, bool)
	ApprovalRequest(string) (projection.ProjectedApprovalRequest, bool)
}

func deriveBoardAndAttention(
	view projection.GlobalReadView,
	teamInstanceID string,
) (TeamBoard, []AttentionItem) {
	execution, _ := view.TeamExecution(teamInstanceID)
	board := TeamBoard{
		schemaVersion:  timelineSchemaVersion,
		teamInstanceID: teamInstanceID,
		planDigest:     execution.PlanDigest,
		status:         execution.Status,
		viewVersion:    view.Version(),
		nodes:          make([]NodeBoardRow, 0, len(execution.Nodes)),
	}
	attention := make([]AttentionItem, 0)
	for _, node := range execution.Nodes {
		row := NodeBoardRow{
			LogicalNodeID:       node.LogicalNodeID,
			Status:              node.Status,
			DependencySatisfied: node.DependencySatisfied,
			CurrentAttempt:      node.CurrentAttempt,
			RecoveryAction:      node.RecoveryAction,
		}
		if !node.RetryAt.IsZero() {
			row.RetryAt = canonicalTime(node.RetryAt)
		}
		if attempt, ok := findProjectedAttempt(
			execution,
			node.LogicalNodeID,
			node.CurrentAttempt,
		); ok {
			row.WorkItemID = attempt.WorkItemID
			row.RunID = attempt.RunID
			row.RuntimeInstanceID = attempt.RuntimeInstanceID
			row.AgentInstanceID = attempt.AgentInstanceID
			if workItem, exists := view.WorkItem(attempt.WorkItemID); exists {
				row.VerificationStatus = workItem.VerificationStatus
			}
			if runtimeRecord, exists := view.RuntimeInstance(attempt.RuntimeInstanceID); exists &&
				runtimeRecord.Status != "online" {
				attention = append(attention, newAttention(
					"runtime_offline",
					"critical",
					teamInstanceID,
					node.LogicalNodeID,
					attempt.WorkItemID,
					"",
					attempt.RuntimeInstanceID,
					runtimeRecord.Status,
					runtimeRecord.StatusChangedAt,
					"restore_runtime",
				))
			}
		}
		board.nodes = append(board.nodes, row)
		if node.Status == "blocked" || node.Status == "human_required" {
			action := "inspect_failure"
			if node.Status == "human_required" {
				action = "provide_input"
			}
			attention = append(attention, newAttention(
				node.Status,
				"critical",
				teamInstanceID,
				node.LogicalNodeID,
				row.WorkItemID,
				"",
				row.RuntimeInstanceID,
				node.Status,
				node.RecoveryDecisionTime,
				action,
			))
		}
	}
	sort.Slice(board.nodes, func(i, j int) bool {
		return board.nodes[i].LogicalNodeID < board.nodes[j].LogicalNodeID
	})
	for _, approval := range view.ApprovalRequestsForTeam(teamInstanceID) {
		if approval.Status != "pending" && approval.Status != "expired" {
			continue
		}
		kind := "pending_approval"
		severity := "warning"
		occurredAt := approval.RequestedAt
		if approval.Status == "expired" {
			kind = "expired_approval"
			severity = "critical"
			occurredAt = approval.DecidedAt
		}
		attention = append(attention, newAttention(
			kind,
			severity,
			teamInstanceID,
			approval.LogicalNodeID,
			approval.WorkItemID,
			approval.ID,
			"",
			approval.Status,
			occurredAt,
			"review_approval",
		))
	}
	for _, workItem := range view.WorkItemsForTeam(teamInstanceID) {
		if workItem.Status == "rejected" ||
			workItem.RecoveryTrigger == "verification_rejected" {
			attention = append(attention, newAttention(
				"verification_failed",
				"critical",
				teamInstanceID,
				workItem.LogicalNodeID,
				workItem.ID,
				"",
				"",
				workItem.Status,
				workItem.AcceptanceDecisionTime,
				"inspect_failure",
			))
		}
	}
	sortAttention(attention)
	return board, attention
}

func newAttention(
	kind,
	severity,
	teamInstanceID,
	logicalNodeID,
	workItemID,
	approvalID,
	runtimeID,
	status string,
	occurredAt time.Time,
	action string,
) AttentionItem {
	occurred := ""
	if !occurredAt.IsZero() {
		occurred = canonicalTime(occurredAt)
	}
	return AttentionItem{
		SchemaVersion: timelineSchemaVersion,
		AttentionID: digestFields(
			"loom.attention.v1",
			kind,
			teamInstanceID,
			logicalNodeID,
			workItemID,
			approvalID,
			runtimeID,
			status,
			occurred,
		),
		Kind:              kind,
		Severity:          severity,
		TeamInstanceID:    teamInstanceID,
		LogicalNodeID:     logicalNodeID,
		WorkItemID:        workItemID,
		ApprovalRequestID: approvalID,
		RuntimeInstanceID: runtimeID,
		Status:            status,
		OccurredAt:        occurred,
		ActionRequired:    action,
	}
}

func attentionFromGap(gap StreamGap) AttentionItem {
	return newAttention(
		"stream_gap",
		"warning",
		gap.teamInstanceID,
		"",
		"",
		"",
		"",
		gap.reason,
		gap.occurredAt,
		"reconnect",
	)
}

func sortAttention(items []AttentionItem) {
	sort.Slice(items, func(i, j int) bool {
		leftCritical := items[i].Severity == "critical"
		rightCritical := items[j].Severity == "critical"
		if leftCritical != rightCritical {
			return leftCritical
		}
		if items[i].OccurredAt != items[j].OccurredAt {
			return items[i].OccurredAt < items[j].OccurredAt
		}
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		return items[i].AttentionID < items[j].AttentionID
	})
}

func findProjectedAttempt(
	execution projection.TeamExecution,
	logicalNodeID string,
	attemptNumber int,
) (projection.TeamExecutionAttempt, bool) {
	for _, node := range execution.Nodes {
		if node.LogicalNodeID != logicalNodeID {
			continue
		}
		for _, attempt := range node.Attempts {
			if attempt.AttemptNumber == attemptNumber {
				return attempt, true
			}
		}
	}
	return projection.TeamExecutionAttempt{}, false
}

func findProjectedAttemptByRunID(
	execution projection.TeamExecution,
	runID string,
) (projection.TeamExecutionAttempt, bool) {
	for _, node := range execution.Nodes {
		for _, attempt := range node.Attempts {
			if attempt.RunID == runID {
				return attempt, true
			}
		}
	}
	return projection.TeamExecutionAttempt{}, false
}

func relatedWorkItemLineage(
	view timelineLineageView,
	execution projection.TeamExecution,
	teamInstanceID,
	workItemID string,
) (string, int, bool) {
	workItem, ok := view.WorkItem(workItemID)
	if !ok || workItem.ID != workItemID {
		return "", 0, false
	}
	for _, node := range execution.Nodes {
		for _, attempt := range node.Attempts {
			if attempt.WorkItemID != workItemID {
				continue
			}
			if workItem.RunID != "" && workItem.RunID != attempt.RunID ||
				workItem.AgentInstanceID != "" &&
					workItem.AgentInstanceID != attempt.AgentInstanceID {
				return "", 0, false
			}
			return node.LogicalNodeID, attempt.AttemptNumber, true
		}
	}
	for _, parent := range view.WorkItemsForTeam(teamInstanceID) {
		if parent.VerifierWorkItemID != workItemID ||
			!validTimelineID(parent.LogicalNodeID) ||
			parent.AttemptNumber <= 0 ||
			parent.VerifierRunID == "" ||
			parent.VerifierRunID != workItem.RunID ||
			parent.VerifierAgentInstanceID == "" ||
			parent.VerifierAgentInstanceID != workItem.AgentInstanceID {
			continue
		}
		attempt, ok := findProjectedAttempt(
			execution,
			parent.LogicalNodeID,
			parent.AttemptNumber,
		)
		if ok && attempt.WorkItemID == parent.ID {
			return parent.LogicalNodeID, parent.AttemptNumber, true
		}
	}
	return "", 0, false
}

func relatedRunLineage(
	view timelineLineageView,
	execution projection.TeamExecution,
	teamInstanceID,
	runID string,
) (string, int, bool) {
	run, ok := view.Run(runID)
	if !ok || run.ID != runID || run.WorkItemID == "" {
		return "", 0, false
	}
	attempt, source := findProjectedAttemptByRunID(execution, runID)
	if source {
		if attempt.WorkItemID != run.WorkItemID {
			return "", 0, false
		}
		for _, node := range execution.Nodes {
			for _, candidate := range node.Attempts {
				if candidate.RunID == runID {
					return node.LogicalNodeID, candidate.AttemptNumber, true
				}
			}
		}
	}
	logicalNodeID, attemptNumber, ok := relatedWorkItemLineage(
		view,
		execution,
		teamInstanceID,
		run.WorkItemID,
	)
	if !ok {
		return "", 0, false
	}
	return logicalNodeID, attemptNumber, true
}

func relatedEvidenceLineage(
	view timelineLineageView,
	execution projection.TeamExecution,
	teamInstanceID,
	evidenceID string,
) (string, int, bool) {
	record, ok := view.Evidence(evidenceID)
	if !ok || record.ID != evidenceID || record.WorkItemID == "" {
		return "", 0, false
	}
	return relatedWorkItemLineage(
		view,
		execution,
		teamInstanceID,
		record.WorkItemID,
	)
}

func relatedApprovalLineage(
	view timelineLineageView,
	execution projection.TeamExecution,
	teamInstanceID,
	approvalID string,
) (string, int, bool) {
	approval, ok := view.ApprovalRequest(approvalID)
	if !ok ||
		approval.ID != approvalID ||
		approval.TeamInstanceID != teamInstanceID ||
		!validTimelineID(approval.LogicalNodeID) ||
		approval.AttemptNumber <= 0 {
		return "", 0, false
	}
	attempt, ok := findProjectedAttempt(
		execution,
		approval.LogicalNodeID,
		approval.AttemptNumber,
	)
	if !ok ||
		approval.WorkItemID != "" &&
			approval.WorkItemID != attempt.WorkItemID ||
		approval.RunID != "" &&
			approval.RunID != attempt.RunID {
		return "", 0, false
	}
	return approval.LogicalNodeID, approval.AttemptNumber, true
}

func resolveDeliveryLineage(
	teamInstanceID string,
	view timelineLineageView,
	event journal.Event,
	kind string,
	safe map[string]string,
) (string, int, error) {
	execution, ok := view.TeamExecution(teamInstanceID)
	if !ok {
		return "", 0, ErrInvalidDeliveryRecord
	}
	logicalNodeID := ""
	attemptNumber := 0
	var found bool
	switch {
	case strings.HasPrefix(event.StreamID, "work-item/"):
		logicalNodeID, attemptNumber, found = relatedWorkItemLineage(
			view,
			execution,
			teamInstanceID,
			strings.TrimPrefix(event.StreamID, "work-item/"),
		)
	case strings.HasPrefix(event.StreamID, "run/"):
		logicalNodeID, attemptNumber, found = relatedRunLineage(
			view,
			execution,
			teamInstanceID,
			strings.TrimPrefix(event.StreamID, "run/"),
		)
	case strings.HasPrefix(event.StreamID, "evidence/"):
		logicalNodeID, attemptNumber, found = relatedEvidenceLineage(
			view,
			execution,
			teamInstanceID,
			strings.TrimPrefix(event.StreamID, "evidence/"),
		)
	case strings.HasPrefix(event.StreamID, "approval/"):
		logicalNodeID, attemptNumber, found = relatedApprovalLineage(
			view,
			execution,
			teamInstanceID,
			strings.TrimPrefix(event.StreamID, "approval/"),
		)
	case event.StreamID == "team-execution/"+teamInstanceID:
		if safe["logical_node_id"] != "" && safe["attempt_number"] != "" {
			attemptNumber, _ = strconv.Atoi(safe["attempt_number"])
			_, found = findProjectedAttempt(
				execution,
				safe["logical_node_id"],
				attemptNumber,
			)
			logicalNodeID = safe["logical_node_id"]
		}
	}
	if err := validateDeliveryLineageFields(
		found,
		logicalNodeID,
		attemptNumber,
		safe,
	); err != nil {
		return "", 0, err
	}
	if found {
		return logicalNodeID, attemptNumber, nil
	}
	switch kind {
	case "team_planned", "ready_set_dispatched", "team_terminal":
		return "", 0, nil
	default:
		return "", 0, ErrInvalidDeliveryRecord
	}
}

func validateDeliveryLineageFields(
	found bool,
	logicalNodeID string,
	attemptNumber int,
	safe map[string]string,
) error {
	if safeLogicalNodeID, present := safe["logical_node_id"]; present {
		if safeLogicalNodeID == "" || !found ||
			logicalNodeID != safeLogicalNodeID {
			return ErrInvalidDeliveryRecord
		}
	}
	if encodedAttempt, present := safe["attempt_number"]; present {
		safeAttempt, err := strconv.Atoi(encodedAttempt)
		if err != nil || safeAttempt <= 0 || !found ||
			attemptNumber != safeAttempt {
			return ErrInvalidDeliveryRecord
		}
	}
	return nil
}

func safeEventFields(payload []byte) (map[string]string, error) {
	var fields map[string]json.RawMessage
	if err := rejectDuplicateJSONKeys(payload); err != nil {
		return nil, ErrInvalidDeliveryRecord
	}
	if err := json.Unmarshal(payload, &fields); err != nil {
		return nil, ErrInvalidDeliveryRecord
	}
	allowed := map[string]struct{}{
		"logical_node_id": {}, "attempt_number": {}, "status": {},
		"reason_code": {}, "reason": {}, "action": {},
		"recovery_action": {}, "warning_code": {}, "retry_at": {},
		"evidence_digest": {}, "source_evidence_digest": {},
		"acceptance_decision_kind": {},
	}
	result := make(map[string]string)
	for key := range allowed {
		raw, ok := fields[key]
		if !ok {
			continue
		}
		if key == "attempt_number" {
			var number int
			if json.Unmarshal(raw, &number) != nil || number < 0 {
				return nil, ErrInvalidDeliveryRecord
			}
			result[key] = strconv.Itoa(number)
			continue
		}
		var value string
		if json.Unmarshal(raw, &value) != nil ||
			!utf8.ValidString(value) ||
			hasForbiddenControl(value) {
			return nil, ErrInvalidDeliveryRecord
		}
		switch key {
		case "retry_at":
			if value != "" {
				parsed, err := time.Parse(time.RFC3339Nano, value)
				if err != nil ||
					parsed.Location() != time.UTC ||
					canonicalTime(parsed) != value {
					return nil, ErrInvalidDeliveryRecord
				}
			}
		case "evidence_digest", "source_evidence_digest":
			if value != "" && !validDigest(value) {
				return nil, ErrInvalidDeliveryRecord
			}
		}
		result[key] = value
	}
	return result, nil
}

func latestTeamArtifactDigest(
	view projection.GlobalReadView,
	teamInstanceID string,
) string {
	workItems := view.WorkItemsForTeam(teamInstanceID)
	sort.Slice(workItems, func(i, j int) bool {
		if !workItems[i].AcceptanceDecisionTime.Equal(
			workItems[j].AcceptanceDecisionTime,
		) {
			return workItems[i].AcceptanceDecisionTime.After(
				workItems[j].AcceptanceDecisionTime,
			)
		}
		return workItems[i].ID < workItems[j].ID
	})
	for _, workItem := range workItems {
		for _, digest := range []string{
			workItem.VerifierEvidenceDigest,
			workItem.SourceEvidenceDigest,
		} {
			if validDigest(digest) {
				return digest
			}
		}
	}
	return ""
}

func normalizeTimelineHeads(
	heads []journal.StreamHead,
) ([]journal.StreamHead, error) {
	if len(heads) == 0 || len(heads) > journal.MaxCursorStreams {
		return nil, ErrInvalidTimelineCursor
	}
	normalized := append([]journal.StreamHead(nil), heads...)
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].StreamID < normalized[j].StreamID
	})
	for index, head := range normalized {
		if !validTimelineID(head.StreamID) ||
			head.Sequence < 0 ||
			head.Sequence == 0 && head.EventID != "" ||
			head.Sequence > 0 && !validTimelineID(head.EventID) ||
			index > 0 && head.StreamID == normalized[index-1].StreamID {
			return nil, ErrInvalidTimelineCursor
		}
	}
	return normalized, nil
}

func validTimelineID(value string) bool {
	return value != "" &&
		len(value) <= 512 &&
		utf8.ValidString(value) &&
		strings.TrimSpace(value) == value &&
		!hasForbiddenControl(value)
}

func validTentativeDelta(value string) bool {
	if value == "" || len(value) > maxTentativeDelta ||
		!utf8.ValidString(value) {
		return false
	}
	for _, current := range value {
		if unicode.IsControl(current) && current != '\n' && current != '\t' {
			return false
		}
	}
	return true
}

func hasForbiddenControl(value string) bool {
	for _, current := range value {
		if unicode.IsControl(current) {
			return true
		}
	}
	return false
}

func validDigest(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && hex.EncodeToString(decoded) == value
}

func validGapReason(reason string) bool {
	switch reason {
	case "invalid_cursor", "cursor_conflict", "scope_overflow",
		"page_overflow", "tentative_overflow":
		return true
	default:
		return false
	}
}

func canonicalTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func writeDigestField(writer io.Writer, value string) {
	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(value)))
	_, _ = writer.Write(length[:])
	_, _ = io.WriteString(writer, value)
}

func digestFields(fields ...string) string {
	hasher := sha256.New()
	for _, field := range fields {
		writeDigestField(hasher, field)
	}
	return hex.EncodeToString(hasher.Sum(nil))
}

func marshalCompact(value any) ([]byte, error) {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	data := bytes.TrimSuffix(buffer.Bytes(), []byte{'\n'})
	return append([]byte(nil), data...), nil
}

func decodeExactJSON(data []byte, target any) error {
	if err := rejectDuplicateJSONKeys(data); err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}

func rejectDuplicateJSONKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := scanJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return fmt.Errorf("trailing JSON")
	}
	return nil
}

func scanJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("invalid JSON object key")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("duplicate JSON key")
			}
			seen[key] = struct{}{}
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return fmt.Errorf("invalid JSON object")
		}
	case '[':
		for decoder.More() {
			if err := scanJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return fmt.Errorf("invalid JSON array")
		}
	default:
		return fmt.Errorf("invalid JSON delimiter")
	}
	return nil
}

func firstNonempty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func cloneBoard(board TeamBoard) TeamBoard {
	board.nodes = append([]NodeBoardRow(nil), board.nodes...)
	return board
}

func cloneTimelinePage(page TimelinePage) TimelinePage {
	page.records = append([]DeliveryRecord(nil), page.records...)
	page.attention = append([]AttentionItem(nil), page.attention...)
	page.board = cloneBoard(page.board)
	if page.gap != nil {
		gap := *page.gap
		page.gap = &gap
	}
	return page
}
