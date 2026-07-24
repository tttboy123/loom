package projection

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"loom-pi-rebuild/internal/journal"
)

var (
	ErrConflictingEvent        = errors.New("conflicting projection event")
	ErrSequenceGap             = errors.New("projection event sequence gap")
	ErrUnsupportedEventVersion = errors.New("unsupported projection event schema version")
	ErrInvalidProjectionEvent  = errors.New("invalid projection event")
)

type source interface {
	Events(context.Context) ([]journal.Event, error)
}

type journalSource struct {
	db *sql.DB
}

func newJournalSource(db *sql.DB) journalSource {
	return journalSource{db: db}
}

func (s journalSource) Events(ctx context.Context) ([]journal.Event, error) {
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

	var events []journal.Event
	for rows.Next() {
		event, err := scanJournalEvent(rows)
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

type Snapshot struct {
	Modes     map[string]string
	WorkItems map[string]WorkItem
	Evidence  map[string]Evidence
}

type WorkItem struct {
	ID     string
	Title  string
	Status string
}

type Evidence struct {
	ID         string
	WorkItemID string
	Digest     string
}

type Projection struct {
	mu          sync.RWMutex
	source      source
	snapshot    Snapshot
	rebuildGate chan struct{}
}

func New(db *sql.DB) *Projection {
	return &Projection{
		source:      newJournalSource(db),
		snapshot:    emptySnapshot(),
		rebuildGate: make(chan struct{}, 1),
	}
}

func (p *Projection) Rebuild(ctx context.Context) error {
	select {
	case p.rebuildGate <- struct{}{}:
		defer func() { <-p.rebuildGate }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	events, err := p.source.Events(ctx)
	if err != nil {
		return err
	}
	candidate, err := replay(ctx, events)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	p.snapshot = candidate.clone()
	return nil
}

func (p *Projection) Snapshot() Snapshot {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.snapshot.clone()
}

func replay(ctx context.Context, events []journal.Event) (Snapshot, error) {
	ordered := append([]journal.Event(nil), events...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].StreamID != ordered[j].StreamID {
			return ordered[i].StreamID < ordered[j].StreamID
		}
		if ordered[i].Seq != ordered[j].Seq {
			return ordered[i].Seq < ordered[j].Seq
		}
		return ordered[i].ID < ordered[j].ID
	})

	candidate := emptySnapshot()
	seenByID := make(map[string]journal.Event)
	seenByStreamSeq := make(map[streamSeq]journal.Event)
	nextSeq := make(map[string]int64)

	for _, event := range ordered {
		if err := ctx.Err(); err != nil {
			return Snapshot{}, err
		}
		if event.SchemaVersion != 1 {
			return Snapshot{}, fmt.Errorf("%w: %d", ErrUnsupportedEventVersion, event.SchemaVersion)
		}
		if existing, ok := seenByID[event.ID]; ok {
			if !sameImmutableEvent(existing, event) {
				return Snapshot{}, fmt.Errorf("%w: id %s", ErrConflictingEvent, event.ID)
			}
		}
		key := streamSeq{streamID: event.StreamID, seq: event.Seq}
		if existing, ok := seenByStreamSeq[key]; ok {
			if !sameImmutableEvent(existing, event) {
				return Snapshot{}, fmt.Errorf("%w: stream %s seq %d", ErrConflictingEvent, event.StreamID, event.Seq)
			}
		}
		if existing, ok := seenByID[event.ID]; ok && sameImmutableEvent(existing, event) {
			continue
		}

		wantSeq := nextSeq[event.StreamID] + 1
		if event.Seq != wantSeq {
			return Snapshot{}, fmt.Errorf("%w: stream %s seq %d want %d", ErrSequenceGap, event.StreamID, event.Seq, wantSeq)
		}
		seenByID[event.ID] = event
		seenByStreamSeq[key] = event
		nextSeq[event.StreamID] = event.Seq

		if err := candidate.apply(event); err != nil {
			return Snapshot{}, err
		}
	}
	return candidate, nil
}

func (s Snapshot) apply(event journal.Event) error {
	switch event.Type {
	case "ModeSelected":
		var payload struct {
			Mode string `json:"mode"`
		}
		if err := decodeRelevantPayload(event, &payload); err != nil {
			return err
		}
		if payload.Mode != "conversation" && payload.Mode != "agent" {
			return fmt.Errorf("%w: invalid mode", ErrInvalidProjectionEvent)
		}
		if existing, ok := s.Modes[event.StreamID]; ok && existing != payload.Mode {
			return fmt.Errorf("%w: conflicting mode", ErrInvalidProjectionEvent)
		}
		s.Modes[event.StreamID] = payload.Mode
	case "WorkItemCreated":
		var payload struct {
			WorkItemID string `json:"work_item_id"`
			Title      string `json:"title"`
			Status     string `json:"status"`
		}
		if err := decodeRelevantPayload(event, &payload); err != nil {
			return err
		}
		if payload.WorkItemID == "" || payload.Title == "" || payload.Status == "" {
			return fmt.Errorf("%w: missing work item create field", ErrInvalidProjectionEvent)
		}
		workItem := WorkItem{ID: payload.WorkItemID, Title: payload.Title, Status: payload.Status}
		if existing, ok := s.WorkItems[payload.WorkItemID]; ok {
			if existing == workItem {
				return nil
			}
			return fmt.Errorf("%w: conflicting work item create", ErrInvalidProjectionEvent)
		}
		s.WorkItems[payload.WorkItemID] = workItem
	case "WorkItemTerminal":
		var payload struct {
			WorkItemID string `json:"work_item_id"`
			Status     string `json:"status"`
		}
		if err := decodeRelevantPayload(event, &payload); err != nil {
			return err
		}
		if payload.WorkItemID == "" || payload.Status == "" {
			return fmt.Errorf("%w: missing work item terminal field", ErrInvalidProjectionEvent)
		}
		workItem, ok := s.WorkItems[payload.WorkItemID]
		if !ok {
			return fmt.Errorf("%w: terminal before create", ErrInvalidProjectionEvent)
		}
		workItem.Status = payload.Status
		s.WorkItems[payload.WorkItemID] = workItem
	case "EvidenceSubmitted":
		var payload struct {
			EvidenceID string `json:"evidence_id"`
			WorkItemID string `json:"work_item_id"`
			Digest     string `json:"digest"`
		}
		if err := decodeRelevantPayload(event, &payload); err != nil {
			return err
		}
		if payload.EvidenceID == "" || payload.WorkItemID == "" || payload.Digest == "" {
			return fmt.Errorf("%w: missing evidence field", ErrInvalidProjectionEvent)
		}
		if !validSHA256Digest(payload.Digest) {
			return fmt.Errorf("%w: invalid evidence digest", ErrInvalidProjectionEvent)
		}
		if _, ok := s.WorkItems[payload.WorkItemID]; !ok {
			return fmt.Errorf("%w: evidence references unknown work item", ErrInvalidProjectionEvent)
		}
		evidence := Evidence{ID: payload.EvidenceID, WorkItemID: payload.WorkItemID, Digest: payload.Digest}
		if existing, ok := s.Evidence[payload.EvidenceID]; ok {
			if existing == evidence {
				return nil
			}
			return fmt.Errorf("%w: conflicting evidence", ErrInvalidProjectionEvent)
		}
		s.Evidence[payload.EvidenceID] = evidence
	default:
		return nil
	}
	return nil
}

func validSHA256Digest(digest string) bool {
	if len(digest) != 64 {
		return false
	}
	for _, ch := range digest {
		switch {
		case ch >= '0' && ch <= '9':
		case ch >= 'a' && ch <= 'f':
		default:
			return false
		}
	}
	return true
}

func decodeRelevantPayload(event journal.Event, target any) error {
	if !json.Valid(event.PayloadJSON) {
		return fmt.Errorf("%w: malformed payload", ErrInvalidProjectionEvent)
	}
	if err := json.Unmarshal(event.PayloadJSON, target); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidProjectionEvent, err)
	}
	return nil
}

func emptySnapshot() Snapshot {
	return Snapshot{
		Modes:     make(map[string]string),
		WorkItems: make(map[string]WorkItem),
		Evidence:  make(map[string]Evidence),
	}
}

func (s Snapshot) clone() Snapshot {
	out := emptySnapshot()
	for id, mode := range s.Modes {
		out.Modes[id] = mode
	}
	for id, workItem := range s.WorkItems {
		out.WorkItems[id] = workItem
	}
	for id, evidence := range s.Evidence {
		out.Evidence[id] = evidence
	}
	return out
}

type streamSeq struct {
	streamID string
	seq      int64
}

type eventScanner interface {
	Scan(dest ...any) error
}

func scanJournalEvent(scanner eventScanner) (journal.Event, error) {
	var event journal.Event
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
		return journal.Event{}, err
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

func sameImmutableEvent(left, right journal.Event) bool {
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
