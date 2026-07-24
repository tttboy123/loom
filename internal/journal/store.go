package journal

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

const supportedSchemaVersion = 1

var (
	ErrInvalidEvent         = errors.New("invalid event")
	ErrUnsupportedVersion   = errors.New("unsupported event schema version")
	ErrIdempotencyConflict  = errors.New("idempotency key reused for different event content")
	ErrSequenceConflict     = errors.New("stream sequence already exists")
	ErrEmptyStreamForReplay = errors.New("empty stream id")
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
	if event.SchemaVersion != supportedSchemaVersion {
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
