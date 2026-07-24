package journal

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestAppendBatchCommitsAllEventsInInputOrder(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	events := batchEvents()

	got, err := store.AppendBatch(ctx, events)
	if err != nil {
		t.Fatalf("AppendBatch() error = %v", err)
	}
	if !reflect.DeepEqual(got, events) {
		t.Fatalf("AppendBatch() = %#v, want %#v", got, events)
	}
	if countEvents(t, ctx, store.db) != len(events) {
		t.Fatalf("event count = %d, want %d", countEvents(t, ctx, store.db), len(events))
	}
	alpha, err := store.ReadStream(ctx, "stream.batch.alpha")
	if err != nil {
		t.Fatalf("ReadStream(alpha) error = %v", err)
	}
	if len(alpha) != 2 || alpha[0].Seq != 1 || alpha[1].Seq != 2 {
		t.Fatalf("alpha stream = %#v", alpha)
	}
	bravo, err := store.ReadStream(ctx, "stream.batch.bravo")
	if err != nil {
		t.Fatalf("ReadStream(bravo) error = %v", err)
	}
	if len(bravo) != 1 || bravo[0].ID != events[2].ID {
		t.Fatalf("bravo stream = %#v", bravo)
	}

	singleStore := newMigratedStore(t)
	single, err := singleStore.AppendBatch(ctx, events[:1])
	if err != nil || len(single) != 1 || !sameImmutableEvent(single[0], events[0]) {
		t.Fatalf("single AppendBatch() = (%#v,%v)", single, err)
	}
}

func TestAppendBatchExactAndReorderedRetry(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	events := batchEvents()
	first, err := store.AppendBatch(ctx, events)
	if err != nil {
		t.Fatalf("first AppendBatch() error = %v", err)
	}
	second, err := store.AppendBatch(ctx, events)
	if err != nil || !reflect.DeepEqual(second, first) {
		t.Fatalf("exact retry = (%#v,%v), want %#v", second, err, first)
	}
	reordered := []Event{events[2], events[0], events[1]}
	third, err := store.AppendBatch(ctx, reordered)
	if err != nil || !reflect.DeepEqual(third, reordered) {
		t.Fatalf("reordered retry = (%#v,%v), want %#v", third, err, reordered)
	}
	if countEvents(t, ctx, store.db) != len(events) {
		t.Fatalf("retry event count = %d, want %d", countEvents(t, ctx, store.db), len(events))
	}
}

func TestAppendBatchConflictsAreAtomic(t *testing.T) {
	ctx := context.Background()

	t.Run("partial existing", func(t *testing.T) {
		store := newMigratedStore(t)
		events := batchEvents()
		if _, err := store.Append(ctx, events[0]); err != nil {
			t.Fatalf("Append(seed) error = %v", err)
		}
		got, err := store.AppendBatch(ctx, events)
		if !errors.Is(err, ErrPartialEventBatchConflict) {
			t.Fatalf("AppendBatch() error = %v", err)
		}
		assertZeroEventBatch(t, got)
		if countEvents(t, ctx, store.db) != 1 {
			t.Fatalf("partial conflict event count = %d, want 1", countEvents(t, ctx, store.db))
		}
	})

	t.Run("idempotency conflict", func(t *testing.T) {
		store := newMigratedStore(t)
		events := batchEvents()
		if _, err := store.Append(ctx, events[0]); err != nil {
			t.Fatalf("Append(seed) error = %v", err)
		}
		changed := withEvent(events[0], func(event *Event) {
			event.PayloadJSON = []byte(`{"value":"changed"}`)
		})
		got, err := store.AppendBatch(ctx, []Event{events[1], changed})
		if !errors.Is(err, ErrIdempotencyConflict) {
			t.Fatalf("AppendBatch() error = %v", err)
		}
		assertZeroEventBatch(t, got)
		if countEvents(t, ctx, store.db) != 1 {
			t.Fatalf("idempotency conflict event count = %d, want 1", countEvents(t, ctx, store.db))
		}
	})

	t.Run("sequence conflict", func(t *testing.T) {
		store := newMigratedStore(t)
		events := batchEvents()
		if _, err := store.Append(ctx, events[0]); err != nil {
			t.Fatalf("Append(seed) error = %v", err)
		}
		conflict := testEvent("evt.batch.conflict", events[0].StreamID, events[0].Seq, "idem.batch.conflict")
		got, err := store.AppendBatch(ctx, []Event{events[1], conflict})
		if !errors.Is(err, ErrSequenceConflict) {
			t.Fatalf("AppendBatch() error = %v", err)
		}
		assertZeroEventBatch(t, got)
		if countEvents(t, ctx, store.db) != 1 {
			t.Fatalf("sequence conflict event count = %d, want 1", countEvents(t, ctx, store.db))
		}
	})
}

func TestAppendBatchConflictClassificationIsOrderIndependent(t *testing.T) {
	ctx := context.Background()
	for _, reverse := range []bool{false, true} {
		store := newMigratedStore(t)
		seedID := testEvent("evt.precedence.idem", "stream.precedence.idem", 1, "idem.precedence")
		seedSeq := testEvent("evt.precedence.seq", "stream.precedence.seq", 1, "idem.precedence.seq.seed")
		if _, err := store.Append(ctx, seedID); err != nil {
			t.Fatalf("Append(idempotency seed) error = %v", err)
		}
		if _, err := store.Append(ctx, seedSeq); err != nil {
			t.Fatalf("Append(sequence seed) error = %v", err)
		}
		idempotencyConflict := withEvent(seedID, func(event *Event) {
			event.PayloadJSON = []byte(`{"value":"changed"}`)
		})
		sequenceConflict := testEvent(
			"evt.precedence.seq.changed",
			seedSeq.StreamID,
			seedSeq.Seq,
			"idem.precedence.seq.changed",
		)
		batch := []Event{sequenceConflict, idempotencyConflict}
		if reverse {
			batch[0], batch[1] = batch[1], batch[0]
		}
		got, err := store.AppendBatch(ctx, batch)
		if !errors.Is(err, ErrIdempotencyConflict) {
			t.Fatalf("reverse=%t AppendBatch() error = %v", reverse, err)
		}
		assertZeroEventBatch(t, got)
		if countEvents(t, ctx, store.db) != 2 {
			t.Fatalf("reverse=%t conflict count = %d, want 2",
				reverse, countEvents(t, ctx, store.db))
		}
	}
}

func TestAppendBatchPreflightRejectsInvalidAndDuplicateInput(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	events := batchEvents()
	oversized := make([]Event, 33)
	for index := range oversized {
		oversized[index] = testEvent(
			fmt.Sprintf("evt.large.%02d", index),
			fmt.Sprintf("stream.large.%02d", index),
			1,
			fmt.Sprintf("idem.large.%02d", index),
		)
	}
	invalidLate := append([]Event(nil), events...)
	invalidLate[2] = withEvent(invalidLate[2], func(event *Event) {
		event.PayloadJSON = []byte(`{`)
	})
	duplicateKey := append([]Event(nil), events[:2]...)
	duplicateKey[1].IdempotencyKey = duplicateKey[0].IdempotencyKey
	duplicateSequence := append([]Event(nil), events[:2]...)
	duplicateSequence[1].StreamID = duplicateSequence[0].StreamID
	duplicateSequence[1].Seq = duplicateSequence[0].Seq

	tests := []struct {
		name   string
		events []Event
		want   error
	}{
		{name: "empty", events: nil, want: ErrInvalidEventBatch},
		{name: "oversized", events: oversized, want: ErrEventBatchTooLarge},
		{name: "late invalid", events: invalidLate, want: ErrInvalidEvent},
		{name: "duplicate idempotency", events: duplicateKey, want: ErrDuplicateBatchIdempotencyKey},
		{name: "duplicate sequence", events: duplicateSequence, want: ErrDuplicateBatchStreamSequence},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := store.AppendBatch(ctx, tt.events)
			if !errors.Is(err, tt.want) {
				t.Fatalf("AppendBatch() error = %v, want %v", err, tt.want)
			}
			assertZeroEventBatch(t, got)
			if countEvents(t, ctx, store.db) != 0 {
				t.Fatalf("preflight failure persisted %d events", countEvents(t, ctx, store.db))
			}
		})
	}
}

func TestAppendBatchRollsBackInsertCancelClosedAndCommitFailures(t *testing.T) {
	ctx := context.Background()
	events := batchEvents()

	t.Run("late insert", func(t *testing.T) {
		store := newMigratedStore(t)
		if _, err := store.db.ExecContext(ctx, `
			CREATE TRIGGER fail_batch_insert
			BEFORE INSERT ON events
			WHEN NEW.id = 'evt.batch.3'
			BEGIN
				SELECT RAISE(ABORT, 'forced batch insert failure');
			END
		`); err != nil {
			t.Fatalf("create trigger error = %v", err)
		}
		got, err := store.AppendBatch(ctx, events)
		if err == nil {
			t.Fatal("AppendBatch() error = nil")
		}
		assertZeroEventBatch(t, got)
		if countEvents(t, ctx, store.db) != 0 {
			t.Fatalf("insert failure persisted %d events", countEvents(t, ctx, store.db))
		}
	})

	t.Run("cancelled", func(t *testing.T) {
		store := newMigratedStore(t)
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		got, err := store.AppendBatch(cancelled, events)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("AppendBatch() error = %v", err)
		}
		assertZeroEventBatch(t, got)
		if countEvents(t, ctx, store.db) != 0 {
			t.Fatalf("cancelled batch persisted %d events", countEvents(t, ctx, store.db))
		}
	})

	t.Run("closed database", func(t *testing.T) {
		store := newMigratedStore(t)
		if err := store.db.Close(); err != nil {
			t.Fatalf("Close() error = %v", err)
		}
		got, err := store.AppendBatch(ctx, events)
		if err == nil {
			t.Fatal("AppendBatch() error = nil")
		}
		assertZeroEventBatch(t, got)
	})

	t.Run("commit", func(t *testing.T) {
		driverName := fmt.Sprintf("journal_batch_commit_%d", time.Now().UnixNano())
		commitErr := errors.New("forced batch commit failure")
		sql.Register(driverName, batchCommitFailureDriver{commitErr: commitErr})
		db, err := sql.Open(driverName, "")
		if err != nil {
			t.Fatalf("sql.Open() error = %v", err)
		}
		t.Cleanup(func() { _ = db.Close() })
		got, err := NewStore(db).AppendBatch(ctx, events[:2])
		if !errors.Is(err, commitErr) {
			t.Fatalf("AppendBatch() error = %v, want %v", err, commitErr)
		}
		assertZeroEventBatch(t, got)
	})
}

func TestAppendBatchConcurrentIdenticalSubmissionsCommitOneFactSet(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	events := batchEvents()
	const workers = 8
	results := make(chan batchAppendResult, workers)
	var start sync.WaitGroup
	start.Add(1)
	for range workers {
		go func() {
			start.Wait()
			got, err := store.AppendBatch(ctx, events)
			results <- batchAppendResult{events: got, err: err}
		}()
	}
	start.Done()

	successes := 0
	for range workers {
		result := <-results
		if result.err != nil {
			assertZeroEventBatch(t, result.events)
			continue
		}
		successes++
		if !reflect.DeepEqual(result.events, events) {
			t.Fatalf("successful concurrent result = %#v, want %#v", result.events, events)
		}
	}
	if successes == 0 {
		t.Fatal("concurrent batches produced no successful caller")
	}
	if countEvents(t, ctx, store.db) != len(events) {
		t.Fatalf("concurrent event count = %d, want %d", countEvents(t, ctx, store.db), len(events))
	}
}

func TestAppendBatchInputAndResultMutationIsolation(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	events := batchEvents()
	originalPayload := append([]byte(nil), events[0].PayloadJSON...)
	got, err := store.AppendBatch(ctx, events)
	if err != nil {
		t.Fatalf("AppendBatch() error = %v", err)
	}
	events[0].PayloadJSON[0] = '['
	got[0].PayloadJSON[0] = '['

	stored, err := store.ReadStream(ctx, "stream.batch.alpha")
	if err != nil {
		t.Fatalf("ReadStream() error = %v", err)
	}
	if !reflect.DeepEqual(stored[0].PayloadJSON, originalPayload) {
		t.Fatalf("stored payload = %q, want %q", stored[0].PayloadJSON, originalPayload)
	}
	retry, err := store.AppendBatch(ctx, batchEvents())
	if err != nil || !reflect.DeepEqual(retry[0].PayloadJSON, originalPayload) {
		t.Fatalf("retry after mutation = (%#v,%v)", retry, err)
	}
}

func TestAppendBatchProductionBoundary(t *testing.T) {
	content, err := os.ReadFile("store.go")
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	source := string(content)
	for _, forbidden := range []string{
		`"net/`, `"os"`, `"path/filepath"`, `"sync"`,
		"CREATE TABLE", "ALTER TABLE", "TeamInstance", "AgentInstance",
		"WorkItem", "AgentGrant", "go func",
	} {
		if strings.Contains(source, forbidden) {
			t.Fatalf("store.go contains forbidden S2-W14 surface %q", forbidden)
		}
	}
}

func batchEvents() []Event {
	return []Event{
		testEvent("evt.batch.1", "stream.batch.alpha", 1, "idem.batch.1"),
		testEvent("evt.batch.2", "stream.batch.alpha", 2, "idem.batch.2"),
		testEvent("evt.batch.3", "stream.batch.bravo", 1, "idem.batch.3"),
	}
}

func assertZeroEventBatch(t *testing.T, got []Event) {
	t.Helper()
	if len(got) != 0 {
		t.Fatalf("failed batch returned Events %#v", got)
	}
}

type batchAppendResult struct {
	events []Event
	err    error
}

type batchCommitFailureDriver struct {
	commitErr error
}

func (d batchCommitFailureDriver) Open(string) (driver.Conn, error) {
	return batchCommitFailureConn{commitErr: d.commitErr}, nil
}

type batchCommitFailureConn struct {
	commitErr error
}

func (c batchCommitFailureConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("Prepare is not implemented")
}

func (c batchCommitFailureConn) Close() error {
	return nil
}

func (c batchCommitFailureConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c batchCommitFailureConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	return batchCommitFailureTx{commitErr: c.commitErr}, nil
}

func (batchCommitFailureConn) ExecContext(
	context.Context,
	string,
	[]driver.NamedValue,
) (driver.Result, error) {
	return driver.RowsAffected(1), nil
}

func (batchCommitFailureConn) QueryContext(
	_ context.Context,
	query string,
	_ []driver.NamedValue,
) (driver.Rows, error) {
	if strings.Contains(query, "idempotency_key = ?") {
		return &emptyBatchRows{columns: []string{
			"id", "stream_id", "seq", "idempotency_key", "event_type",
			"schema_version", "emitted_at", "correlation_id", "causation_id",
			"payload_json",
		}}, nil
	}
	return &emptyBatchRows{columns: []string{"found"}}, nil
}

type batchCommitFailureTx struct {
	commitErr error
}

func (tx batchCommitFailureTx) Commit() error {
	return tx.commitErr
}

func (batchCommitFailureTx) Rollback() error {
	return nil
}

type emptyBatchRows struct {
	columns []string
}

func (r *emptyBatchRows) Columns() []string {
	return r.columns
}

func (*emptyBatchRows) Close() error {
	return nil
}

func (*emptyBatchRows) Next([]driver.Value) error {
	return io.EOF
}
