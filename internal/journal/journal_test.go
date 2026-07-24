package journal

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
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
		{name: "unsupported version", event: withEvent(valid, func(e *Event) { e.SchemaVersion = 2 })},
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
