package journal

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"
)

const (
	maxSupportedSchemaVersion = 2
	maxEventBatchSize         = 32
	maxAtomicStreamSetSize    = 64
	MaxCursorStreams          = 96
	MaxReadPageEvents         = 128
)

var (
	ErrInvalidEvent                 = errors.New("invalid event")
	ErrUnsupportedVersion           = errors.New("unsupported event schema version")
	ErrIdempotencyConflict          = errors.New("idempotency key reused for different event content")
	ErrSequenceConflict             = errors.New("stream sequence already exists")
	ErrEmptyStreamForReplay         = errors.New("empty stream id")
	ErrInvalidEventBatch            = errors.New("invalid event batch")
	ErrEventBatchTooLarge           = errors.New("event batch too large")
	ErrDuplicateBatchIdempotencyKey = errors.New("duplicate batch idempotency key")
	ErrDuplicateBatchStreamSequence = errors.New("duplicate batch stream sequence")
	ErrPartialEventBatchConflict    = errors.New("partial event batch conflict")
	ErrStreamHeadConflict           = errors.New("stream head conflict")
	ErrInvalidStreamCursor          = errors.New("invalid stream cursor")
	ErrStreamCursorConflict         = errors.New("stream cursor conflict")
	ErrStreamSequenceGap            = errors.New("stream sequence gap")
	ErrStreamPageLimit              = errors.New("stream page limit")
)

type Event struct {
	ID             string
	StreamID       string
	Seq            int64
	IdempotencyKey string
	Type           string
	SchemaVersion  int
	EmittedAt      time.Time
	CorrelationID  string
	CausationID    string
	PayloadJSON    []byte
}

type Store struct {
	db *sql.DB
}

type StreamHeadExpectation struct {
	StreamID string
	Sequence int64
}

type StreamHead struct {
	StreamID string
	Sequence int64
	EventID  string
}

type StreamSetSnapshot struct {
	events []Event
	heads  []StreamHead
}

type StreamPage struct {
	events  []Event
	heads   []StreamHead
	hasMore bool
}

func (page StreamPage) Events() []Event {
	return cloneJournalEvents(page.events)
}

func (page StreamPage) Heads() []StreamHead {
	return append([]StreamHead(nil), page.heads...)
}

func (page StreamPage) HasMore() bool {
	return page.hasMore
}

func (snapshot StreamSetSnapshot) Events() []Event {
	return cloneJournalEvents(snapshot.events)
}

func (snapshot StreamSetSnapshot) Heads() []StreamHead {
	return append([]StreamHead(nil), snapshot.heads...)
}

func (snapshot StreamSetSnapshot) Head(streamID string) (StreamHead, bool) {
	index := sort.Search(len(snapshot.heads), func(index int) bool {
		return snapshot.heads[index].StreamID >= streamID
	})
	if index == len(snapshot.heads) || snapshot.heads[index].StreamID != streamID {
		return StreamHead{}, false
	}
	return snapshot.heads[index], true
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) Append(ctx context.Context, event Event) (Event, error) {
	normalized, err := validateEvent(event)
	if err != nil {
		return Event{}, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Event{}, err
	}
	inserted, insertErr := insertEvent(ctx, tx, normalized)
	if insertErr == nil {
		if err := tx.Commit(); err != nil {
			return Event{}, err
		}
		return inserted, nil
	}

	existingByKey, foundByKey, lookupErr := findByIdempotencyKey(ctx, tx, normalized.IdempotencyKey)
	if lookupErr != nil {
		_ = tx.Rollback()
		return Event{}, insertErr
	}
	if foundByKey {
		if sameImmutableEvent(existingByKey, normalized) {
			if err := tx.Commit(); err != nil {
				return Event{}, err
			}
			return existingByKey, nil
		}
		_ = tx.Rollback()
		return Event{}, fmt.Errorf("%w: %s", ErrIdempotencyConflict, normalized.IdempotencyKey)
	}

	foundBySequence, lookupErr := existsStreamSequence(ctx, tx, normalized.StreamID, normalized.Seq)
	if lookupErr != nil {
		_ = tx.Rollback()
		return Event{}, insertErr
	}
	if foundBySequence {
		_ = tx.Rollback()
		return Event{}, fmt.Errorf("%w: %s/%d", ErrSequenceConflict, normalized.StreamID, normalized.Seq)
	}

	_ = tx.Rollback()
	return Event{}, insertErr
}

func (s *Store) AppendBatch(ctx context.Context, events []Event) ([]Event, error) {
	normalized, err := normalizeEventBatch(events)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	rollback := func() {
		_ = tx.Rollback()
	}

	existing := make([]Event, len(normalized))
	exactCount := 0
	var idempotencyConflict string
	var sequenceConflict eventStreamSequence
	for index, event := range normalized {
		committed, found, lookupErr := findByIdempotencyKey(ctx, tx, event.IdempotencyKey)
		if lookupErr != nil {
			rollback()
			return nil, lookupErr
		}
		if found {
			if sameImmutableEvent(committed, event) {
				existing[index] = cloneJournalEvent(committed)
				exactCount++
			} else if idempotencyConflict == "" {
				idempotencyConflict = event.IdempotencyKey
			}
			continue
		}
		occupied, lookupErr := existsStreamSequence(ctx, tx, event.StreamID, event.Seq)
		if lookupErr != nil {
			rollback()
			return nil, lookupErr
		}
		if occupied && sequenceConflict.StreamID == "" {
			sequenceConflict = eventStreamSequence{
				StreamID: event.StreamID,
				Seq:      event.Seq,
			}
		}
	}

	if idempotencyConflict != "" {
		rollback()
		return nil, fmt.Errorf("%w: %s", ErrIdempotencyConflict, idempotencyConflict)
	}
	if sequenceConflict.StreamID != "" {
		rollback()
		return nil, fmt.Errorf(
			"%w: %s/%d",
			ErrSequenceConflict,
			sequenceConflict.StreamID,
			sequenceConflict.Seq,
		)
	}
	if exactCount > 0 && exactCount < len(normalized) {
		rollback()
		return nil, ErrPartialEventBatchConflict
	}
	if exactCount == len(normalized) {
		if err := ctx.Err(); err != nil {
			rollback()
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return cloneJournalEvents(existing), nil
	}

	inserted := make([]Event, len(normalized))
	for index, event := range normalized {
		current, insertErr := insertEvent(ctx, tx, event)
		if insertErr != nil {
			rollback()
			return nil, insertErr
		}
		inserted[index] = cloneJournalEvent(current)
	}
	if err := ctx.Err(); err != nil {
		rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return cloneJournalEvents(inserted), nil
}

func (s *Store) AppendBatchIfStreamHeads(
	ctx context.Context,
	expectations []StreamHeadExpectation,
	events []Event,
) ([]Event, error) {
	normalizedHeads, err := normalizeStreamHeadExpectations(expectations)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeEventBatch(events)
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	rollback := func() {
		_ = tx.Rollback()
	}
	if _, err := tx.ExecContext(ctx, `UPDATE events SET seq = seq WHERE 0`); err != nil {
		rollback()
		return nil, err
	}

	existing := make([]Event, len(normalized))
	exactCount := 0
	var idempotencyConflict string
	var sequenceConflict eventStreamSequence
	for index, event := range normalized {
		committed, found, lookupErr := findByIdempotencyKey(ctx, tx, event.IdempotencyKey)
		if lookupErr != nil {
			rollback()
			return nil, lookupErr
		}
		if found {
			if sameImmutableEvent(committed, event) {
				existing[index] = cloneJournalEvent(committed)
				exactCount++
			} else if idempotencyConflict == "" {
				idempotencyConflict = event.IdempotencyKey
			}
			continue
		}
		occupied, lookupErr := existsStreamSequence(ctx, tx, event.StreamID, event.Seq)
		if lookupErr != nil {
			rollback()
			return nil, lookupErr
		}
		if occupied && sequenceConflict.StreamID == "" {
			sequenceConflict = eventStreamSequence{
				StreamID: event.StreamID,
				Seq:      event.Seq,
			}
		}
	}

	if idempotencyConflict != "" {
		rollback()
		return nil, fmt.Errorf("%w: %s", ErrIdempotencyConflict, idempotencyConflict)
	}
	if exactCount > 0 && exactCount < len(normalized) {
		rollback()
		return nil, ErrPartialEventBatchConflict
	}
	if exactCount == len(normalized) {
		if err := ctx.Err(); err != nil {
			rollback()
			return nil, err
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return cloneJournalEvents(existing), nil
	}

	for _, expectation := range normalizedHeads {
		var current int64
		if err := tx.QueryRowContext(
			ctx,
			`SELECT COALESCE(MAX(seq), 0) FROM events WHERE stream_id = ?`,
			expectation.StreamID,
		).Scan(&current); err != nil {
			rollback()
			return nil, err
		}
		if current != expectation.Sequence {
			rollback()
			return nil, ErrStreamHeadConflict
		}
	}
	if sequenceConflict.StreamID != "" {
		rollback()
		return nil, fmt.Errorf(
			"%w: %s/%d",
			ErrSequenceConflict,
			sequenceConflict.StreamID,
			sequenceConflict.Seq,
		)
	}

	inserted := make([]Event, len(normalized))
	for index, event := range normalized {
		current, insertErr := insertEvent(ctx, tx, event)
		if insertErr != nil {
			rollback()
			return nil, insertErr
		}
		inserted[index] = cloneJournalEvent(current)
	}
	if err := ctx.Err(); err != nil {
		rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return cloneJournalEvents(inserted), nil
}

func (s *Store) ReadStream(ctx context.Context, streamID string) ([]Event, error) {
	if streamID == "" {
		return nil, ErrEmptyStreamForReplay
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, stream_id, seq, idempotency_key, event_type, schema_version,
		       emitted_at, correlation_id, causation_id, payload_json
		FROM events
		WHERE stream_id = ?
		ORDER BY seq ASC
	`, streamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []Event
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return events, nil
}

func (s *Store) ReadStreamSet(
	ctx context.Context,
	streamIDs []string,
) (StreamSetSnapshot, error) {
	normalized, err := normalizeStreamIDs(streamIDs)
	if err != nil {
		return StreamSetSnapshot{}, err
	}
	if err := ctx.Err(); err != nil {
		return StreamSetSnapshot{}, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return StreamSetSnapshot{}, err
	}
	rollback := func() {
		_ = tx.Rollback()
	}
	events := make([]Event, 0)
	heads := make([]StreamHead, 0, len(normalized))
	for _, streamID := range normalized {
		if err := ctx.Err(); err != nil {
			rollback()
			return StreamSetSnapshot{}, err
		}
		rows, queryErr := tx.QueryContext(ctx, `
			SELECT id, stream_id, seq, idempotency_key, event_type, schema_version,
			       emitted_at, correlation_id, causation_id, payload_json
			FROM events
			WHERE stream_id = ?
			ORDER BY seq ASC, id ASC
		`, streamID)
		if queryErr != nil {
			rollback()
			return StreamSetSnapshot{}, queryErr
		}
		head := StreamHead{StreamID: streamID}
		for rows.Next() {
			event, scanErr := scanEvent(rows)
			if scanErr != nil {
				_ = rows.Close()
				rollback()
				return StreamSetSnapshot{}, scanErr
			}
			events = append(events, cloneJournalEvent(event))
			head.Sequence = event.Seq
			head.EventID = event.ID
		}
		rowsErr := rows.Err()
		closeErr := rows.Close()
		if rowsErr != nil {
			rollback()
			return StreamSetSnapshot{}, rowsErr
		}
		if closeErr != nil {
			rollback()
			return StreamSetSnapshot{}, closeErr
		}
		heads = append(heads, head)
	}
	if err := ctx.Err(); err != nil {
		rollback()
		return StreamSetSnapshot{}, err
	}
	if err := tx.Commit(); err != nil {
		return StreamSetSnapshot{}, err
	}
	return StreamSetSnapshot{
		events: cloneJournalEvents(events),
		heads:  append([]StreamHead(nil), heads...),
	}, nil
}

func (s *Store) ReadPageAfterHeads(
	ctx context.Context,
	heads []StreamHead,
	limit int,
) (StreamPage, error) {
	normalized, err := normalizeCursorHeads(heads)
	if err != nil {
		return StreamPage{}, err
	}
	if limit < 1 || limit > MaxReadPageEvents {
		return StreamPage{}, ErrStreamPageLimit
	}
	if err := ctx.Err(); err != nil {
		return StreamPage{}, err
	}
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return StreamPage{}, err
	}
	rollback := func() {
		_ = tx.Rollback()
	}
	candidates := make([]Event, 0, len(normalized)*(limit+1))
	for _, head := range normalized {
		if err := ctx.Err(); err != nil {
			rollback()
			return StreamPage{}, err
		}
		if head.Sequence > 0 {
			var eventID string
			err := tx.QueryRowContext(
				ctx,
				`SELECT id FROM events WHERE stream_id = ? AND seq = ?`,
				head.StreamID,
				head.Sequence,
			).Scan(&eventID)
			if errors.Is(err, sql.ErrNoRows) || err == nil && eventID != head.EventID {
				rollback()
				return StreamPage{}, fmt.Errorf(
					"%w: %s/%d",
					ErrStreamCursorConflict,
					head.StreamID,
					head.Sequence,
				)
			}
			if err != nil {
				rollback()
				return StreamPage{}, err
			}
		}
		rows, err := tx.QueryContext(ctx, `
			SELECT id, stream_id, seq, idempotency_key, event_type, schema_version,
			       emitted_at, correlation_id, causation_id, payload_json
			FROM events
			WHERE stream_id = ? AND seq > ?
			ORDER BY seq ASC, id ASC
			LIMIT ?
		`, head.StreamID, head.Sequence, limit+1)
		if err != nil {
			rollback()
			return StreamPage{}, err
		}
		expected := head.Sequence + 1
		for rows.Next() {
			event, scanErr := scanEvent(rows)
			if scanErr != nil {
				_ = rows.Close()
				rollback()
				return StreamPage{}, scanErr
			}
			if event.Seq != expected {
				_ = rows.Close()
				rollback()
				return StreamPage{}, fmt.Errorf(
					"%w: %s/%d",
					ErrStreamSequenceGap,
					event.StreamID,
					event.Seq,
				)
			}
			expected++
			candidates = append(candidates, cloneJournalEvent(event))
		}
		rowsErr := rows.Err()
		closeErr := rows.Close()
		if rowsErr != nil {
			rollback()
			return StreamPage{}, rowsErr
		}
		if closeErr != nil {
			rollback()
			return StreamPage{}, closeErr
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		left, right := candidates[i], candidates[j]
		if !left.EmittedAt.Equal(right.EmittedAt) {
			return left.EmittedAt.Before(right.EmittedAt)
		}
		if left.StreamID != right.StreamID {
			return left.StreamID < right.StreamID
		}
		if left.Seq != right.Seq {
			return left.Seq < right.Seq
		}
		return left.ID < right.ID
	})
	count := len(candidates)
	if count > limit {
		count = limit
	}
	selected := cloneJournalEvents(candidates[:count])
	nextHeads := append([]StreamHead(nil), normalized...)
	headIndex := make(map[string]int, len(nextHeads))
	for index, head := range nextHeads {
		headIndex[head.StreamID] = index
	}
	for _, event := range selected {
		index := headIndex[event.StreamID]
		if event.Seq != nextHeads[index].Sequence+1 {
			rollback()
			return StreamPage{}, fmt.Errorf(
				"%w: non-prefix merge %s/%d",
				ErrStreamSequenceGap,
				event.StreamID,
				event.Seq,
			)
		}
		nextHeads[index].Sequence = event.Seq
		nextHeads[index].EventID = event.ID
	}
	if err := ctx.Err(); err != nil {
		rollback()
		return StreamPage{}, err
	}
	if err := tx.Commit(); err != nil {
		return StreamPage{}, err
	}
	return StreamPage{
		events:  selected,
		heads:   nextHeads,
		hasMore: len(candidates) > count,
	}, nil
}

func (s *Store) ReadAll(ctx context.Context) ([]Event, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, stream_id, seq, idempotency_key, event_type, schema_version,
		       emitted_at, correlation_id, causation_id, payload_json
		FROM events
		ORDER BY stream_id ASC, seq ASC, id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]Event, 0)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, cloneJournalEvent(event))
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return cloneJournalEvents(events), nil
}

type eventStreamSequence struct {
	StreamID string
	Seq      int64
}

func normalizeStreamHeadExpectations(
	expectations []StreamHeadExpectation,
) ([]StreamHeadExpectation, error) {
	if len(expectations) == 0 || len(expectations) > maxAtomicStreamSetSize {
		return nil, ErrInvalidEventBatch
	}
	normalized := make([]StreamHeadExpectation, len(expectations))
	seen := make(map[string]struct{}, len(expectations))
	for index, expectation := range expectations {
		if expectation.StreamID == "" || expectation.Sequence < 0 {
			return nil, ErrInvalidEventBatch
		}
		if _, exists := seen[expectation.StreamID]; exists {
			return nil, ErrInvalidEventBatch
		}
		seen[expectation.StreamID] = struct{}{}
		normalized[index] = expectation
	}
	return normalized, nil
}

func normalizeStreamIDs(streamIDs []string) ([]string, error) {
	if len(streamIDs) == 0 || len(streamIDs) > maxAtomicStreamSetSize {
		return nil, ErrInvalidEventBatch
	}
	normalized := append([]string(nil), streamIDs...)
	sort.Strings(normalized)
	for index, streamID := range normalized {
		if streamID == "" || index > 0 && streamID == normalized[index-1] {
			return nil, ErrInvalidEventBatch
		}
	}
	return normalized, nil
}

func normalizeCursorHeads(heads []StreamHead) ([]StreamHead, error) {
	if len(heads) == 0 || len(heads) > MaxCursorStreams {
		return nil, ErrInvalidStreamCursor
	}
	normalized := append([]StreamHead(nil), heads...)
	sort.Slice(normalized, func(i, j int) bool {
		return normalized[i].StreamID < normalized[j].StreamID
	})
	for index, head := range normalized {
		if head.StreamID == "" ||
			head.Sequence < 0 ||
			head.Sequence == 0 && head.EventID != "" ||
			head.Sequence > 0 && head.EventID == "" ||
			index > 0 && head.StreamID == normalized[index-1].StreamID {
			return nil, ErrInvalidStreamCursor
		}
	}
	return normalized, nil
}

func normalizeEventBatch(events []Event) ([]Event, error) {
	if len(events) == 0 {
		return nil, ErrInvalidEventBatch
	}
	if len(events) > maxEventBatchSize {
		return nil, ErrEventBatchTooLarge
	}
	normalized := make([]Event, len(events))
	idempotencyKeys := make(map[string]struct{}, len(events))
	sequences := make(map[eventStreamSequence]struct{}, len(events))
	for index, event := range events {
		current, err := validateEvent(event)
		if err != nil {
			return nil, err
		}
		if _, exists := idempotencyKeys[current.IdempotencyKey]; exists {
			return nil, fmt.Errorf(
				"%w: %s",
				ErrDuplicateBatchIdempotencyKey,
				current.IdempotencyKey,
			)
		}
		idempotencyKeys[current.IdempotencyKey] = struct{}{}
		key := eventStreamSequence{StreamID: current.StreamID, Seq: current.Seq}
		if _, exists := sequences[key]; exists {
			return nil, fmt.Errorf(
				"%w: %s/%d",
				ErrDuplicateBatchStreamSequence,
				current.StreamID,
				current.Seq,
			)
		}
		sequences[key] = struct{}{}
		normalized[index] = cloneJournalEvent(current)
	}
	return normalized, nil
}

func cloneJournalEvents(events []Event) []Event {
	if len(events) == 0 {
		return []Event{}
	}
	copied := make([]Event, len(events))
	for index, event := range events {
		copied[index] = cloneJournalEvent(event)
	}
	return copied
}

func cloneJournalEvent(event Event) Event {
	event.PayloadJSON = append([]byte(nil), event.PayloadJSON...)
	return event
}

func validateEvent(event Event) (Event, error) {
	if event.ID == "" {
		return Event{}, fmt.Errorf("%w: empty id", ErrInvalidEvent)
	}
	if event.StreamID == "" {
		return Event{}, fmt.Errorf("%w: empty stream id", ErrInvalidEvent)
	}
	if event.Seq <= 0 {
		return Event{}, fmt.Errorf("%w: nonpositive stream sequence", ErrInvalidEvent)
	}
	if event.IdempotencyKey == "" {
		return Event{}, fmt.Errorf("%w: empty idempotency key", ErrInvalidEvent)
	}
	if event.Type == "" {
		return Event{}, fmt.Errorf("%w: empty event type", ErrInvalidEvent)
	}
	if event.SchemaVersion < 1 || event.SchemaVersion > maxSupportedSchemaVersion {
		return Event{}, fmt.Errorf("%w: %d", ErrUnsupportedVersion, event.SchemaVersion)
	}
	if event.EmittedAt.IsZero() {
		return Event{}, fmt.Errorf("%w: zero emitted timestamp", ErrInvalidEvent)
	}
	if len(event.PayloadJSON) == 0 {
		return Event{}, fmt.Errorf("%w: empty payload", ErrInvalidEvent)
	}
	if !json.Valid(event.PayloadJSON) {
		return Event{}, fmt.Errorf("%w: invalid payload json", ErrInvalidEvent)
	}
	event.EmittedAt = event.EmittedAt.UTC()
	return event, nil
}

func insertEvent(ctx context.Context, tx *sql.Tx, event Event) (Event, error) {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO events (
			id, stream_id, seq, idempotency_key, event_type, schema_version,
			emitted_at, correlation_id, causation_id, payload_json
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		event.ID,
		event.StreamID,
		event.Seq,
		event.IdempotencyKey,
		event.Type,
		event.SchemaVersion,
		event.EmittedAt.UnixNano(),
		nullableString(event.CorrelationID),
		nullableString(event.CausationID),
		string(event.PayloadJSON),
	)
	if err != nil {
		return Event{}, err
	}
	return event, nil
}

func findByIdempotencyKey(ctx context.Context, tx *sql.Tx, key string) (Event, bool, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, stream_id, seq, idempotency_key, event_type, schema_version,
		       emitted_at, correlation_id, causation_id, payload_json
		FROM events
		WHERE idempotency_key = ?
	`, key)
	event, err := scanEvent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, false, nil
	}
	if err != nil {
		return Event{}, false, err
	}
	return event, true, nil
}

func existsStreamSequence(ctx context.Context, tx *sql.Tx, streamID string, seq int64) (bool, error) {
	var found int
	err := tx.QueryRowContext(ctx, `
		SELECT 1
		FROM events
		WHERE stream_id = ? AND seq = ?
	`, streamID, seq).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

type eventScanner interface {
	Scan(dest ...any) error
}

func scanEvent(scanner eventScanner) (Event, error) {
	var event Event
	var emittedAt int64
	var correlationID sql.NullString
	var causationID sql.NullString
	var payload string
	if err := scanner.Scan(
		&event.ID,
		&event.StreamID,
		&event.Seq,
		&event.IdempotencyKey,
		&event.Type,
		&event.SchemaVersion,
		&emittedAt,
		&correlationID,
		&causationID,
		&payload,
	); err != nil {
		return Event{}, err
	}
	event.EmittedAt = time.Unix(0, emittedAt).UTC()
	if correlationID.Valid {
		event.CorrelationID = correlationID.String
	}
	if causationID.Valid {
		event.CausationID = causationID.String
	}
	event.PayloadJSON = []byte(payload)
	return event, nil
}

func sameImmutableEvent(left, right Event) bool {
	return left.ID == right.ID &&
		left.StreamID == right.StreamID &&
		left.Seq == right.Seq &&
		left.IdempotencyKey == right.IdempotencyKey &&
		left.Type == right.Type &&
		left.SchemaVersion == right.SchemaVersion &&
		left.EmittedAt.Equal(right.EmittedAt) &&
		left.CorrelationID == right.CorrelationID &&
		left.CausationID == right.CausationID &&
		bytes.Equal(left.PayloadJSON, right.PayloadJSON)
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}
