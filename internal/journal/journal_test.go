package journal

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"loom-pi-rebuild/migrations"

	_ "modernc.org/sqlite"
)

func TestMigrateCreatesAppendOnlySchemaIdempotently(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}

	var eventTableSQL string
	err := db.QueryRowContext(ctx, `
		SELECT sql
		FROM sqlite_schema
		WHERE type = 'table' AND name = 'events'
	`).Scan(&eventTableSQL)
	if err != nil {
		t.Fatalf("events schema lookup error = %v", err)
	}
	for _, want := range []string{
		"idempotency_key TEXT NOT NULL UNIQUE",
		"schema_version INTEGER NOT NULL",
		"UNIQUE (stream_id, seq)",
	} {
		if !containsSQL(eventTableSQL, want) {
			t.Fatalf("events schema %q missing %q", eventTableSQL, want)
		}
	}

	for _, trigger := range []string{"events_no_update", "events_no_delete"} {
		var found int
		err := db.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM sqlite_schema
			WHERE type = 'trigger' AND name = ?
		`, trigger).Scan(&found)
		if err != nil {
			t.Fatalf("trigger lookup %s error = %v", trigger, err)
		}
		if found != 1 {
			t.Fatalf("trigger %s count = %d, want 1", trigger, found)
		}
	}
}

func TestMigrateCreatesNoFutureSliceTables(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	rows, err := db.QueryContext(ctx, `
		SELECT name
		FROM sqlite_schema
		WHERE type = 'table'
		  AND name NOT LIKE 'sqlite_%'
		ORDER BY name
	`)
	if err != nil {
		t.Fatalf("table lookup error = %v", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatalf("scan table error = %v", err)
		}
		tables = append(tables, table)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("table rows error = %v", err)
	}

	want := []string{"events", "schema_migrations"}
	if fmt.Sprint(tables) != fmt.Sprint(want) {
		t.Fatalf("migration tables = %v, want %v", tables, want)
	}
}

func TestMigrateUsesVersionedSQLFileAsSingleSource(t *testing.T) {
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller() failed")
	}
	source, err := os.ReadFile(filepath.Join(filepath.Dir(filename), "migration.go"))
	if err != nil {
		t.Fatalf("read migration.go error = %v", err)
	}
	if strings.Contains(string(source), "CREATE TABLE") {
		t.Fatal("migration.go contains inline DDL, want embedded migrations/0001_init.sql as executable source")
	}
	if !strings.Contains(migrations.InitSQL(), "CREATE TABLE IF NOT EXISTS events") {
		t.Fatal("embedded migration source is missing events DDL")
	}
}

func TestMigrateEnablesForeignKeysOnTransactionConnection(t *testing.T) {
	ctx := context.Background()
	driverName := fmt.Sprintf("journal_migrate_conn_%d", time.Now().UnixNano())
	recorder := &recordingDriver{}
	sql.Register(driverName, recorder)
	db, err := sql.Open(driverName, "")
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	db.SetMaxIdleConns(0)
	t.Cleanup(func() {
		_ = db.Close()
	})

	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	pragmaConn, beginConn := recorder.observedConnections()
	if pragmaConn == 0 || beginConn == 0 {
		t.Fatalf("recorded pragma conn = %d, begin conn = %d; want both recorded", pragmaConn, beginConn)
	}
	if pragmaConn != beginConn {
		t.Fatalf("foreign_keys pragma conn = %d, transaction conn = %d; want same connection", pragmaConn, beginConn)
	}
}

func TestMigrateClosedDatabaseReturnsError(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if err := Migrate(ctx, db); err == nil {
		t.Fatal("Migrate() error = nil, want closed database error")
	}
}

func TestAppendValidatesFrozenEventContract(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	valid := testEvent("evt-validation", "stream-validation", 1, "idem-validation")

	tests := []struct {
		name  string
		event Event
	}{
		{name: "empty id", event: withEvent(valid, func(e *Event) { e.ID = "" })},
		{name: "empty stream", event: withEvent(valid, func(e *Event) { e.StreamID = "" })},
		{name: "nonpositive seq", event: withEvent(valid, func(e *Event) { e.Seq = 0 })},
		{name: "empty idempotency key", event: withEvent(valid, func(e *Event) { e.IdempotencyKey = "" })},
		{name: "empty type", event: withEvent(valid, func(e *Event) { e.Type = "" })},
		{name: "unsupported version", event: withEvent(valid, func(e *Event) { e.SchemaVersion = 3 })},
		{name: "zero emitted at", event: withEvent(valid, func(e *Event) { e.EmittedAt = time.Time{} })},
		{name: "empty payload", event: withEvent(valid, func(e *Event) { e.PayloadJSON = nil })},
		{name: "invalid payload json", event: withEvent(valid, func(e *Event) { e.PayloadJSON = []byte(`{`) })},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := store.Append(ctx, tt.event); err == nil {
				t.Fatal("Append() error = nil, want validation error")
			}
		})
	}
}

func TestAppendAcceptsSupportedSchemaVersionTwo(t *testing.T) {
	store := newMigratedStore(t)
	event := testEvent("evt-schema-v2", "stream-schema-v2", 1, "idem-schema-v2")
	event.SchemaVersion = 2
	if _, err := store.Append(context.Background(), event); err != nil {
		t.Fatalf("Append() schema v2 error = %v", err)
	}
}

func TestAppendPersistsAllImmutableFields(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	event := Event{
		ID:             "evt-1",
		StreamID:       "work-1",
		Seq:            1,
		IdempotencyKey: "message-1",
		Type:           "WorkItemCreated",
		SchemaVersion:  1,
		EmittedAt:      time.Date(2026, 7, 24, 4, 5, 6, 0, time.FixedZone("SGT", 8*60*60)),
		CorrelationID:  "corr-1",
		CausationID:    "cause-1",
		PayloadJSON:    []byte(`{"work_item_id":"work-1"}`),
	}

	got, err := store.Append(ctx, event)
	if err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	wantEmittedAt := event.EmittedAt.UTC()
	if got.ID != event.ID ||
		got.StreamID != event.StreamID ||
		got.Seq != event.Seq ||
		got.IdempotencyKey != event.IdempotencyKey ||
		got.Type != event.Type ||
		got.SchemaVersion != event.SchemaVersion ||
		!got.EmittedAt.Equal(wantEmittedAt) ||
		got.CorrelationID != event.CorrelationID ||
		got.CausationID != event.CausationID ||
		string(got.PayloadJSON) != string(event.PayloadJSON) {
		t.Fatalf("Append() = %#v, want immutable fields from %#v with UTC timestamp", got, event)
	}
}

func TestAppendSameIdempotencyKeyAndSameContentReturnsOriginalFact(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	event := testEvent("evt-idem", "stream-idem", 1, "idem-key")

	first, err := store.Append(ctx, event)
	if err != nil {
		t.Fatalf("first Append() error = %v", err)
	}
	second, err := store.Append(ctx, event)
	if err != nil {
		t.Fatalf("second Append() error = %v", err)
	}

	if first.ID != second.ID || first.StreamID != second.StreamID || first.Seq != second.Seq {
		t.Fatalf("second Append() = %#v, want original fact %#v", second, first)
	}
	if got := countEvents(t, ctx, store.db); got != 1 {
		t.Fatalf("event row count = %d, want 1", got)
	}
}

func TestAppendSameIdempotencyKeyDifferentContentFailsClosed(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	event := testEvent("evt-conflict", "stream-conflict", 1, "idem-conflict")

	if _, err := store.Append(ctx, event); err != nil {
		t.Fatalf("first Append() error = %v", err)
	}
	changed := withEvent(event, func(e *Event) {
		e.PayloadJSON = []byte(`{"value":"changed"}`)
	})

	if _, err := store.Append(ctx, changed); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("second Append() error = %v, want ErrIdempotencyConflict", err)
	}
	if got := countEvents(t, ctx, store.db); got != 1 {
		t.Fatalf("event row count = %d, want 1", got)
	}
}

func TestAppendRejectsDirectSQLUpdateAndDelete(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	event := testEvent("evt-append-only", "stream-append-only", 1, "idem-append-only")
	if _, err := store.Append(ctx, event); err != nil {
		t.Fatalf("Append() error = %v", err)
	}

	if _, err := store.db.ExecContext(ctx, `UPDATE events SET event_type = 'Changed' WHERE id = ?`, event.ID); err == nil {
		t.Fatal("direct UPDATE error = nil, want append-only enforcement error")
	}
	if _, err := store.db.ExecContext(ctx, `DELETE FROM events WHERE id = ?`, event.ID); err == nil {
		t.Fatal("direct DELETE error = nil, want append-only enforcement error")
	}
	if got := countEvents(t, ctx, store.db); got != 1 {
		t.Fatalf("event row count = %d, want 1", got)
	}
}

func TestAppendConcurrentDuplicateSubmissionCreatesOneFact(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	event := testEvent("evt-concurrent", "stream-concurrent", 1, "idem-concurrent")

	const workers = 12
	results := make(chan appendResult, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := store.Append(ctx, event)
			results <- appendResult{event: got, err: err}
		}()
	}
	wg.Wait()
	close(results)

	for result := range results {
		if result.err != nil {
			t.Fatalf("concurrent Append() error = %v", result.err)
		}
		if result.event.ID != event.ID || result.event.StreamID != event.StreamID || result.event.Seq != event.Seq {
			t.Fatalf("concurrent Append() = %#v, want original fact", result.event)
		}
	}
	if got := countEvents(t, ctx, store.db); got != 1 {
		t.Fatalf("event row count = %d, want 1", got)
	}
}

func TestAppendConflictingStreamSequenceFailsWithoutExtraRow(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	first := testEvent("evt-seq-1", "stream-seq", 1, "idem-seq-1")
	conflict := testEvent("evt-seq-2", "stream-seq", 1, "idem-seq-2")

	if _, err := store.Append(ctx, first); err != nil {
		t.Fatalf("first Append() error = %v", err)
	}
	if _, err := store.Append(ctx, conflict); !errors.Is(err, ErrSequenceConflict) {
		t.Fatalf("conflicting Append() error = %v, want ErrSequenceConflict", err)
	}
	if got := countEvents(t, ctx, store.db); got != 1 {
		t.Fatalf("event row count = %d, want 1", got)
	}
}

func TestAppendConcurrentStreamSequenceConflictCreatesOneFact(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)

	const workers = 12
	results := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			event := testEvent(
				fmt.Sprintf("evt-seq-race-%d", i),
				"stream-seq-race",
				1,
				fmt.Sprintf("idem-seq-race-%d", i),
			)
			_, err := store.Append(ctx, event)
			results <- err
		}(i)
	}
	wg.Wait()
	close(results)

	var successes int
	var conflicts int
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrSequenceConflict):
			conflicts++
		default:
			t.Fatalf("concurrent sequence Append() error = %v, want nil or ErrSequenceConflict", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful appends = %d, want 1", successes)
	}
	if conflicts != workers-1 {
		t.Fatalf("sequence conflicts = %d, want %d", conflicts, workers-1)
	}
	if got := countEvents(t, ctx, store.db); got != 1 {
		t.Fatalf("event row count = %d, want 1", got)
	}
}

func TestAppendClosedDatabaseReturnsErrorWithoutSuccessFallback(t *testing.T) {
	ctx := context.Background()
	db := openTestDB(t)
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	store := NewStore(db)
	if err := db.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if _, err := store.Append(ctx, testEvent("evt-closed", "stream-closed", 1, "idem-closed")); err == nil {
		t.Fatal("Append() error = nil, want closed database error")
	}
}

func TestReadStreamReturnsDeterministicSequenceOrder(t *testing.T) {
	ctx := context.Background()
	store := newMigratedStore(t)
	for _, event := range []Event{
		testEvent("evt-read-2", "stream-read", 2, "idem-read-2"),
		testEvent("evt-read-1", "stream-read", 1, "idem-read-1"),
		testEvent("evt-other", "stream-other", 1, "idem-other"),
	} {
		if _, err := store.Append(ctx, event); err != nil {
			t.Fatalf("Append(%s) error = %v", event.ID, err)
		}
	}

	events, err := store.ReadStream(ctx, "stream-read")
	if err != nil {
		t.Fatalf("ReadStream() error = %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("ReadStream() length = %d, want 2", len(events))
	}
	if events[0].Seq != 1 || events[1].Seq != 2 {
		t.Fatalf("ReadStream() seqs = [%d, %d], want [1, 2]", events[0].Seq, events[1].Seq)
	}
}

type appendResult struct {
	event Event
	err   error
}

func newMigratedStore(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()
	db := openTestDB(t)
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	return NewStore(db)
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "journal.db")
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	values.Add("_pragma", "journal_mode(WAL)")
	db, err := sql.Open("sqlite", fmt.Sprintf("file:%s?%s", dbPath, values.Encode()))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
	})
	return db
}

func testEvent(id, streamID string, seq int64, idempotencyKey string) Event {
	return Event{
		ID:             id,
		StreamID:       streamID,
		Seq:            seq,
		IdempotencyKey: idempotencyKey,
		Type:           "RunEventRecorded",
		SchemaVersion:  1,
		EmittedAt:      time.Date(2026, 7, 24, 1, 2, 3, 0, time.UTC),
		PayloadJSON:    []byte(`{"value":"stable"}`),
	}
}

func withEvent(event Event, mutate func(*Event)) Event {
	mutate(&event)
	return event
}

func countEvents(t *testing.T, ctx context.Context, db *sql.DB) int {
	t.Helper()
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events`).Scan(&count); err != nil {
		t.Fatalf("count events error = %v", err)
	}
	return count
}

func containsSQL(sqlText, want string) bool {
	for start := 0; start+len(want) <= len(sqlText); start++ {
		if sqlText[start:start+len(want)] == want {
			return true
		}
	}
	return false
}

func TestAppendBatchIfStreamHeadsContract(t *testing.T) { // s3_w2_journal_multi_stream_head_cas
	t.Parallel()

	t.Run("zero and exact heads commit atomically", func(t *testing.T) {
		store := newMigratedStore(t)
		ctx := context.Background()
		seed := testEvent("evt-cas-seed", "stream-a", 1, "idem-cas-seed")
		if _, err := store.Append(ctx, seed); err != nil {
			t.Fatal(err)
		}

		events := []Event{
			testEvent("evt-cas-a2", "stream-a", 2, "idem-cas-a2"),
			testEvent("evt-cas-b1", "stream-b", 1, "idem-cas-b1"),
		}
		expectations := []StreamHeadExpectation{
			{StreamID: "stream-a", Sequence: 1},
			{StreamID: "stream-b", Sequence: 0},
		}
		committed, err := store.AppendBatchIfStreamHeads(ctx, expectations, events)
		if err != nil {
			t.Fatalf("AppendBatchIfStreamHeads() error = %v", err)
		}
		if !reflect.DeepEqual(committed, events) {
			t.Fatalf("committed = %#v, want %#v", committed, events)
		}

		expectations[0].StreamID = "mutated"
		events[0].PayloadJSON[0] = '['
		committed[1].PayloadJSON[0] = '['
		gotA, err := store.ReadStream(ctx, "stream-a")
		if err != nil {
			t.Fatal(err)
		}
		gotB, err := store.ReadStream(ctx, "stream-b")
		if err != nil {
			t.Fatal(err)
		}
		if len(gotA) != 2 || len(gotB) != 1 ||
			string(gotA[1].PayloadJSON) != `{"value":"stable"}` ||
			string(gotB[0].PayloadJSON) != `{"value":"stable"}` {
			t.Fatalf("committed facts alias inputs/results: a=%#v b=%#v", gotA, gotB)
		}
	})

	t.Run("identical retry precedes stale head rejection", func(t *testing.T) {
		store := newMigratedStore(t)
		ctx := context.Background()
		events := []Event{
			testEvent("evt-retry-a1", "retry-a", 1, "idem-retry-a1"),
			testEvent("evt-retry-b1", "retry-b", 1, "idem-retry-b1"),
		}
		heads := []StreamHeadExpectation{
			{StreamID: "retry-a", Sequence: 0},
			{StreamID: "retry-b", Sequence: 0},
		}
		first, err := store.AppendBatchIfStreamHeads(ctx, heads, events)
		if err != nil {
			t.Fatal(err)
		}
		second, err := store.AppendBatchIfStreamHeads(ctx, heads, events)
		if err != nil {
			t.Fatalf("exact retry error = %v", err)
		}
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("retry changed facts: first=%#v second=%#v", first, second)
		}

		conflicting := append([]Event(nil), events...)
		conflicting[1] = withEvent(conflicting[1], func(event *Event) {
			event.PayloadJSON = []byte(`{"value":"different"}`)
		})
		if _, err := store.AppendBatchIfStreamHeads(ctx, heads, conflicting); !errors.Is(err, ErrIdempotencyConflict) {
			t.Fatalf("conflicting retry error = %v, want ErrIdempotencyConflict", err)
		}
	})

	t.Run("head mismatch is typed and atomic", func(t *testing.T) {
		store := newMigratedStore(t)
		ctx := context.Background()
		seed := testEvent("evt-head-seed", "head-a", 1, "idem-head-seed")
		if _, err := store.Append(ctx, seed); err != nil {
			t.Fatal(err)
		}
		events := []Event{
			testEvent("evt-head-a2", "head-a", 2, "idem-head-a2"),
			testEvent("evt-head-b1", "head-b", 1, "idem-head-b1"),
		}
		for _, heads := range [][]StreamHeadExpectation{
			{{StreamID: "head-a", Sequence: 0}, {StreamID: "head-b", Sequence: 0}},
			{{StreamID: "head-a", Sequence: 2}, {StreamID: "head-b", Sequence: 0}},
			{{StreamID: "head-a", Sequence: 1}, {StreamID: "head-b", Sequence: 1}},
		} {
			if _, err := store.AppendBatchIfStreamHeads(ctx, heads, events); !errors.Is(err, ErrStreamHeadConflict) {
				t.Errorf("heads %#v error = %v, want ErrStreamHeadConflict", heads, err)
			}
			gotA, readErr := store.ReadStream(ctx, "head-a")
			if readErr != nil {
				t.Fatal(readErr)
			}
			gotB, readErr := store.ReadStream(ctx, "head-b")
			if readErr != nil {
				t.Fatal(readErr)
			}
			if len(gotA) != 1 || len(gotB) != 0 {
				t.Fatalf("mismatch partially committed: a=%d b=%d", len(gotA), len(gotB))
			}
		}
	})

	t.Run("expectation validation precedes database work", func(t *testing.T) {
		store := newMigratedStore(t)
		event := testEvent("evt-invalid-head", "invalid-head", 1, "idem-invalid-head")
		cases := [][]StreamHeadExpectation{
			nil,
			{},
			{{StreamID: "", Sequence: 0}},
			{{StreamID: "x", Sequence: -1}},
			{{StreamID: "x", Sequence: 0}, {StreamID: "x", Sequence: 0}},
			func() []StreamHeadExpectation {
				heads := make([]StreamHeadExpectation, maxAtomicStreamSetSize+1)
				for index := range heads {
					heads[index].StreamID = fmt.Sprintf("too-many-%02d", index)
				}
				return heads
			}(),
		}
		for index, heads := range cases {
			if _, err := store.AppendBatchIfStreamHeads(context.Background(), heads, []Event{event}); !errors.Is(err, ErrInvalidEventBatch) {
				t.Errorf("case %d error = %v, want ErrInvalidEventBatch", index, err)
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := store.AppendBatchIfStreamHeads(ctx, []StreamHeadExpectation{{StreamID: "invalid-head"}}, []Event{event}); !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled error = %v, want context.Canceled", err)
		}
	})

	t.Run("concurrent contenders linearize on expected head", func(t *testing.T) {
		store := newMigratedStore(t)
		ctx := context.Background()
		seed := testEvent("evt-race-seed", "race-stream", 1, "idem-race-seed")
		if _, err := store.Append(ctx, seed); err != nil {
			t.Fatal(err)
		}

		start := make(chan struct{})
		results := make(chan error, 2)
		for index := 0; index < 2; index++ {
			index := index
			go func() {
				<-start
				event := testEvent(
					fmt.Sprintf("evt-race-%d", index),
					"race-stream",
					2,
					fmt.Sprintf("idem-race-%d", index),
				)
				_, err := store.AppendBatchIfStreamHeads(
					ctx,
					[]StreamHeadExpectation{{StreamID: "race-stream", Sequence: 1}},
					[]Event{event},
				)
				results <- err
			}()
		}
		close(start)
		var success, conflicts int
		for index := 0; index < 2; index++ {
			err := <-results
			switch {
			case err == nil:
				success++
			case errors.Is(err, ErrStreamHeadConflict):
				conflicts++
			default:
				t.Fatalf("unexpected contender error = %v", err)
			}
		}
		if success != 1 || conflicts != 1 {
			t.Fatalf("success=%d conflicts=%d, want 1/1", success, conflicts)
		}
		events, err := store.ReadStream(ctx, "race-stream")
		if err != nil {
			t.Fatal(err)
		}
		if len(events) != 2 {
			t.Fatalf("event count = %d, want 2", len(events))
		}
	})
}

func TestReadAllDeterministicIsolatedAndCancelable(t *testing.T) {
	t.Parallel()
	store := newMigratedStore(t)
	ctx := context.Background()

	empty, err := store.ReadAll(ctx)
	if err != nil {
		t.Fatalf("empty ReadAll() error = %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Fatalf("empty ReadAll() = %#v, want non-nil empty", empty)
	}

	input := []Event{
		testEvent("evt-z2", "z-stream", 2, "idem-z2"),
		testEvent("evt-a1", "a-stream", 1, "idem-a1"),
		testEvent("evt-z1", "z-stream", 1, "idem-z1"),
	}
	if _, err := store.AppendBatch(ctx, input); err != nil {
		t.Fatal(err)
	}
	got, err := store.ReadAll(ctx)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	if len(got) != 3 ||
		got[0].ID != "evt-a1" ||
		got[1].ID != "evt-z1" ||
		got[2].ID != "evt-z2" {
		t.Fatalf("ReadAll() order = %#v", got)
	}
	got[0].PayloadJSON[0] = '['
	again, err := store.ReadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if string(again[0].PayloadJSON) != `{"value":"stable"}` {
		t.Fatal("ReadAll result aliases committed payload")
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if events, err := store.ReadAll(canceled); !errors.Is(err, context.Canceled) || events != nil {
		t.Fatalf("canceled ReadAll() = %#v, %v", events, err)
	}
}

func TestReadStreamSetReturnsCanonicalDeepCopiedEventsAndEveryHead(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()
	first := testEvent("11111111-1111-4111-8111-111111111111", "stream-b", 1, "idem-set-1")
	second := testEvent("22222222-2222-4222-8222-222222222222", "stream-a", 1, "idem-set-2")
	third := testEvent("33333333-3333-4333-8333-333333333333", "stream-a", 2, "idem-set-3")
	for _, event := range []Event{first, second, third} {
		if _, err := store.Append(ctx, event); err != nil {
			t.Fatalf("Append(%s) error = %v", event.ID, err)
		}
	}

	snapshot, err := store.ReadStreamSet(ctx, []string{"stream-b", "stream-empty", "stream-a"})
	if err != nil {
		t.Fatalf("ReadStreamSet() error = %v", err)
	}
	events := snapshot.Events()
	if got, want := []string{events[0].ID, events[1].ID, events[2].ID},
		[]string{second.ID, third.ID, first.ID}; !reflect.DeepEqual(got, want) {
		t.Fatalf("event order = %v, want %v", got, want)
	}
	heads := snapshot.Heads()
	if got, want := heads, []StreamHead{
		{StreamID: "stream-a", Sequence: 2, EventID: third.ID},
		{StreamID: "stream-b", Sequence: 1, EventID: first.ID},
		{StreamID: "stream-empty", Sequence: 0, EventID: ""},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Heads() = %#v, want %#v", got, want)
	}
	if head, ok := snapshot.Head("stream-empty"); !ok || head.Sequence != 0 || head.EventID != "" {
		t.Fatalf("empty stream head = %#v, %v", head, ok)
	}

	events[0].PayloadJSON[0] = 'x'
	heads[0].StreamID = "mutated"
	again, err := store.ReadStreamSet(ctx, []string{"stream-a", "stream-b", "stream-empty"})
	if err != nil {
		t.Fatalf("second ReadStreamSet() error = %v", err)
	}
	if !json.Valid(again.Events()[0].PayloadJSON) || again.Heads()[0].StreamID != "stream-a" {
		t.Fatal("caller mutation escaped the stream-set snapshot")
	}
}

func TestReadStreamSetValidationCancellationAndBoundedHeadCASLimit(t *testing.T) {
	store := newMigratedStore(t)
	if _, err := store.ReadStreamSet(context.Background(), nil); !errors.Is(err, ErrInvalidEventBatch) {
		t.Fatalf("empty ReadStreamSet() error = %v", err)
	}
	if _, err := store.ReadStreamSet(context.Background(), []string{"same", "same"}); !errors.Is(err, ErrInvalidEventBatch) {
		t.Fatalf("duplicate ReadStreamSet() error = %v", err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := store.ReadStreamSet(cancelled, []string{"stream-a"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled ReadStreamSet() error = %v", err)
	}

	expectations := make([]StreamHeadExpectation, maxAtomicStreamSetSize)
	for index := range expectations {
		expectations[index] = StreamHeadExpectation{
			StreamID: fmt.Sprintf("stream-%02d", index),
			Sequence: 0,
		}
	}
	event := testEvent("44444444-4444-4444-8444-444444444444", "stream-00", 1, "idem-set-4")
	if _, err := store.AppendBatchIfStreamHeads(context.Background(), expectations, []Event{event}); err != nil {
		t.Fatalf("bounded head CAS error = %v", err)
	}
	expectations = append(expectations, StreamHeadExpectation{StreamID: "stream-over-limit", Sequence: 0})
	event = testEvent("55555555-5555-4555-8555-555555555555", "stream-over-limit", 1, "idem-set-5")
	if _, err := store.AppendBatchIfStreamHeads(context.Background(), expectations, []Event{event}); !errors.Is(err, ErrInvalidEventBatch) {
		t.Fatalf("over-limit head CAS error = %v", err)
	}
}

func TestReadPageAfterHeadsReturnsDeterministicContiguousCopiedPrefix(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()
	base := time.Date(2026, 7, 26, 8, 0, 0, 0, time.UTC)
	a1 := withEvent(
		testEvent("61111111-1111-4111-8111-111111111111", "stream-a", 1, "page-a-1"),
		func(event *Event) { event.EmittedAt = base },
	)
	a2 := withEvent(
		testEvent("62222222-2222-4222-8222-222222222222", "stream-a", 2, "page-a-2"),
		func(event *Event) { event.EmittedAt = base.Add(2 * time.Second) },
	)
	b1 := withEvent(
		testEvent("63333333-3333-4333-8333-333333333333", "stream-b", 1, "page-b-1"),
		func(event *Event) { event.EmittedAt = base.Add(time.Second) },
	)
	for _, event := range []Event{a1, a2, b1} {
		if _, err := store.Append(ctx, event); err != nil {
			t.Fatalf("Append(%s) error = %v", event.ID, err)
		}
	}

	page, err := store.ReadPageAfterHeads(ctx, []StreamHead{
		{StreamID: "stream-b"},
		{StreamID: "stream-a"},
	}, 2)
	if err != nil {
		t.Fatalf("ReadPageAfterHeads() error = %v", err)
	}
	if got, want := page.Events(), []Event{a1, b1}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Events() = %#v, want %#v", got, want)
	}
	if !page.HasMore() {
		t.Fatal("HasMore() = false, want true")
	}
	if got, want := page.Heads(), []StreamHead{
		{StreamID: "stream-a", Sequence: 1, EventID: a1.ID},
		{StreamID: "stream-b", Sequence: 1, EventID: b1.ID},
	}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Heads() = %#v, want %#v", got, want)
	}

	mutatedEvents := page.Events()
	mutatedHeads := page.Heads()
	mutatedEvents[0].PayloadJSON[0] = '['
	mutatedHeads[0].StreamID = "mutated"
	if !json.Valid(page.Events()[0].PayloadJSON) ||
		page.Heads()[0].StreamID != "stream-a" {
		t.Fatal("StreamPage accessors alias caller mutation")
	}

	next, err := store.ReadPageAfterHeads(ctx, page.Heads(), 2)
	if err != nil {
		t.Fatalf("second ReadPageAfterHeads() error = %v", err)
	}
	if got := next.Events(); len(got) != 1 || got[0].ID != a2.ID ||
		next.HasMore() {
		t.Fatalf("second page = %#v hasMore=%v", got, next.HasMore())
	}
}

func TestReadPageAfterHeadsRejectsInvalidConflictGapBoundsAndCancellation(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()
	first := testEvent(
		"64444444-4444-4444-8444-444444444444",
		"stream-a",
		1,
		"page-invalid-1",
	)
	if _, err := store.Append(ctx, first); err != nil {
		t.Fatal(err)
	}

	invalid := []struct {
		name  string
		heads []StreamHead
		limit int
		want  error
	}{
		{name: "empty", limit: 1, want: ErrInvalidStreamCursor},
		{name: "duplicate", heads: []StreamHead{{StreamID: "a"}, {StreamID: "a"}}, limit: 1, want: ErrInvalidStreamCursor},
		{name: "zero with id", heads: []StreamHead{{StreamID: "a", EventID: "event"}}, limit: 1, want: ErrInvalidStreamCursor},
		{name: "positive without id", heads: []StreamHead{{StreamID: "a", Sequence: 1}}, limit: 1, want: ErrInvalidStreamCursor},
		{name: "limit zero", heads: []StreamHead{{StreamID: "a"}}, limit: 0, want: ErrStreamPageLimit},
		{name: "limit high", heads: []StreamHead{{StreamID: "a"}}, limit: MaxReadPageEvents + 1, want: ErrStreamPageLimit},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			if _, err := store.ReadPageAfterHeads(ctx, test.heads, test.limit); !errors.Is(err, test.want) {
				t.Fatalf("error = %v, want errors.Is(%v)", err, test.want)
			}
		})
	}

	tooMany := make([]StreamHead, MaxCursorStreams+1)
	for index := range tooMany {
		tooMany[index].StreamID = fmt.Sprintf("stream-%03d", index)
	}
	if _, err := store.ReadPageAfterHeads(ctx, tooMany, 1); !errors.Is(err, ErrInvalidStreamCursor) {
		t.Fatalf("too-many-stream error = %v", err)
	}
	if _, err := store.ReadPageAfterHeads(ctx, []StreamHead{{
		StreamID: "stream-a", Sequence: 1,
		EventID: "65555555-5555-4555-8555-555555555555",
	}}, 1); !errors.Is(err, ErrStreamCursorConflict) {
		t.Fatalf("head conflict error = %v", err)
	}

	gapped := testEvent(
		"66666666-6666-4666-8666-666666666666",
		"stream-gap",
		2,
		"page-gap-2",
	)
	if _, err := store.Append(ctx, gapped); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadPageAfterHeads(ctx, []StreamHead{{StreamID: "stream-gap"}}, 1); !errors.Is(err, ErrStreamSequenceGap) {
		t.Fatalf("sequence-gap error = %v", err)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := store.ReadPageAfterHeads(cancelled, []StreamHead{{StreamID: "stream-a"}}, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled error = %v", err)
	}
}

func TestReadPageAfterHeadsConcurrentAppendIsVisibleNowOrFromReturnedCursor(t *testing.T) {
	store := newMigratedStore(t)
	ctx := context.Background()
	for iteration := 0; iteration < 50; iteration++ {
		streamID := fmt.Sprintf("concurrent-page-%03d", iteration)
		event := testEvent(
			fmt.Sprintf("70000000-0000-4000-8000-%012d", iteration),
			streamID,
			1,
			fmt.Sprintf("concurrent-page-%03d", iteration),
		)
		start := make(chan struct{})
		appendResult := make(chan error, 1)
		go func() {
			<-start
			_, err := store.Append(ctx, event)
			appendResult <- err
		}()
		close(start)
		page, err := store.ReadPageAfterHeads(
			ctx,
			[]StreamHead{{StreamID: streamID}},
			1,
		)
		if err != nil {
			t.Fatalf("iteration %d ReadPageAfterHeads() error = %v", iteration, err)
		}
		if err := <-appendResult; err != nil {
			t.Fatalf("iteration %d Append() error = %v", iteration, err)
		}
		events := page.Events()
		switch len(events) {
		case 0:
			next, err := store.ReadPageAfterHeads(ctx, page.Heads(), 1)
			if err != nil {
				t.Fatalf("iteration %d next page error = %v", iteration, err)
			}
			events = next.Events()
		case 1:
		default:
			t.Fatalf("iteration %d page Events() = %#v", iteration, events)
		}
		if len(events) != 1 || events[0].ID != event.ID {
			t.Fatalf("iteration %d delivered Events() = %#v", iteration, events)
		}
	}
}

type recordingDriver struct {
	mu         sync.Mutex
	nextID     int
	pragmaConn int
	beginConn  int
}

func (d *recordingDriver) Open(_ string) (driver.Conn, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.nextID++
	return &recordingConn{driver: d, id: d.nextID}, nil
}

func (d *recordingDriver) observedConnections() (int, int) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.pragmaConn, d.beginConn
}

type recordingConn struct {
	driver *recordingDriver
	id     int
}

func (c *recordingConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("Prepare is not implemented by recordingConn")
}

func (c *recordingConn) Close() error {
	return nil
}

func (c *recordingConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *recordingConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.driver.mu.Lock()
	c.driver.beginConn = c.id
	c.driver.mu.Unlock()
	return recordingTx{}, nil
}

func (c *recordingConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	if strings.TrimSpace(query) == "PRAGMA foreign_keys = ON" {
		c.driver.mu.Lock()
		c.driver.pragmaConn = c.id
		c.driver.mu.Unlock()
	}
	return driver.RowsAffected(0), nil
}

type recordingTx struct{}

func (recordingTx) Commit() error {
	return nil
}

func (recordingTx) Rollback() error {
	return nil
}
