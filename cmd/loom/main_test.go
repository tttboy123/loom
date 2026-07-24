package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/mode"
	"loom-pi-rebuild/internal/projection"

	_ "modernc.org/sqlite"
)

func TestRunRouteDistinguishesConversationAndExplicitAgentTriggers(t *testing.T) {
	tests := []struct {
		trigger string
		want    string
	}{
		{trigger: "plain_input", want: "conversation"},
		{trigger: "use_agent", want: "agent"},
		{trigger: "select_agent", want: "agent"},
		{trigger: "select_team", want: "agent"},
		{trigger: "assign", want: "agent"},
		{trigger: "unknown_trigger", want: "conversation"},
	}

	for _, tt := range tests {
		t.Run(tt.trigger, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			deps := testDeps()
			code := run(context.Background(), []string{"route", "--trigger", tt.trigger, "--target", "target-1", "--text", "hello"}, &stdout, &stderr, deps.runDeps)

			if code != 0 {
				t.Fatalf("run route exit = %d, want 0; stderr=%q", code, stderr.String())
			}
			if got := stdout.String(); got != fmt.Sprintf("{\"command\":\"route\",\"mode\":\"%s\"}\n", tt.want) {
				t.Fatalf("stdout = %q, want route JSON for %s", got, tt.want)
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
			if deps.statusCalls != 0 {
				t.Fatalf("status calls = %d, want 0 for route", deps.statusCalls)
			}
		})
	}
}

func TestRunStatusReadsOnlyProjectionInDeterministicOrder(t *testing.T) {
	ctx := context.Background()
	dbPath := createJournalDB(t)
	writeProjectionFacts(t, ctx, dbPath)

	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"status", "--state", dbPath}, &stdout, &stderr, productionDeps())

	if code != 0 {
		t.Fatalf("run status exit = %d, want 0; stderr=%q", code, stderr.String())
	}
	want := "{\"command\":\"status\",\"modes\":[{\"stream\":\"mode-a\",\"mode\":\"conversation\"},{\"stream\":\"mode-b\",\"mode\":\"agent\"}],\"work_items\":[{\"id\":\"work-a\",\"title\":\"Alpha\",\"status\":\"accepted\"},{\"id\":\"work-b\",\"title\":\"Beta\",\"status\":\"open\"}],\"evidence\":[{\"id\":\"evidence-a\",\"work_item_id\":\"work-a\",\"digest\":\"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"},{\"id\":\"evidence-b\",\"work_item_id\":\"work-b\",\"digest\":\"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\"}]}\n"
	if got := stdout.String(); got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertCLIDoesNotQueryJournalSQL(t)
	assertReadOnlyDBRejectsWrites(t, ctx, dbPath)
}

func TestRunStatusReadsSpecialPathJournalFilenames(t *testing.T) {
	tests := []string{
		"journal?query.db",
		"journal#fragment.db",
		"journal%percent.db",
		"journal space.db",
	}

	for _, filename := range tests {
		t.Run(filename, func(t *testing.T) {
			dbPath := createJournalDBNamed(t, filename)

			var stdout, stderr bytes.Buffer
			code := run(context.Background(), []string{"status", "--state", dbPath}, &stdout, &stderr, productionDeps())

			if code != 0 {
				t.Fatalf("run status exit = %d, want 0; stderr=%q", code, stderr.String())
			}
			if got := stdout.String(); got != "{\"command\":\"status\",\"modes\":[],\"work_items\":[],\"evidence\":[]}\n" {
				t.Fatalf("stdout = %q, want empty status JSON", got)
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}

	t.Run("missing special path does not leak", func(t *testing.T) {
		assertStateUnavailable(t, context.Background(), filepath.Join(t.TempDir(), "missing?journal#%.db"))
	})
}

func TestRunRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "no command", args: nil},
		{name: "unknown command", args: []string{"unknown"}},
		{name: "missing trigger", args: []string{"route"}},
		{name: "unexpected route positional", args: []string{"route", "--trigger", "plain_input", "extra"}},
		{name: "unexpected route flag", args: []string{"route", "--trigger", "plain_input", "--bad"}},
		{name: "missing state path", args: []string{"status"}},
		{name: "unexpected status positional", args: []string{"status", "--state", "journal.db", "extra"}},
		{name: "unexpected status flag", args: []string{"status", "--state", "journal.db", "--bad"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(context.Background(), tt.args, &stdout, &stderr, testDeps().runDeps)

			if code != 2 {
				t.Fatalf("run(%v) exit = %d, want 2", tt.args, code)
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want empty", stdout.String())
			}
			if !strings.HasPrefix(stderr.String(), "invalid input:") {
				t.Fatalf("stderr = %q, want invalid input prefix", stderr.String())
			}
		})
	}
}

func TestRunStatusUnavailableFailsClearlyWithoutEmptySuccess(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		assertStateUnavailable(t, context.Background(), filepath.Join(t.TempDir(), "missing.db"))
	})
	t.Run("schema absence", func(t *testing.T) {
		dbPath := createEmptySQLiteDB(t)
		assertStateUnavailable(t, context.Background(), dbPath)
	})
	t.Run("replay failure", func(t *testing.T) {
		ctx := context.Background()
		dbPath := createJournalDB(t)
		db := openWritableDB(t, dbPath)
		defer db.Close()
		if _, err := db.ExecContext(ctx, `INSERT INTO events (id, stream_id, seq, idempotency_key, event_type, schema_version, emitted_at, payload_json) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, "evt-gap", "stream-gap", 2, "idem-gap", "ModeSelected", 1, time.Now().UnixNano(), `{"mode":"agent"}`); err != nil {
			t.Fatalf("direct gap insert error = %v", err)
		}
		assertStateUnavailable(t, ctx, dbPath)
	})
	t.Run("canceled context", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		dbPath := createJournalDB(t)
		assertStateUnavailable(t, ctx, dbPath)
	})
}

func TestRunHasNoBackgroundOrNetworkSideEffects(t *testing.T) {
	deps := testDeps()
	var routeOut, routeErr bytes.Buffer
	code := run(context.Background(), []string{"route", "--trigger", "use_agent"}, &routeOut, &routeErr, deps.runDeps)
	if code != 0 {
		t.Fatalf("route exit = %d, want 0; stderr=%q", code, routeErr.String())
	}
	if deps.routeCalls != 1 || deps.statusCalls != 0 {
		t.Fatalf("route calls = %d, status calls = %d; want route only", deps.routeCalls, deps.statusCalls)
	}

	deps = testDeps()
	var statusOut, statusErr bytes.Buffer
	code = run(context.Background(), []string{"status", "--state", "state.db"}, &statusOut, &statusErr, deps.runDeps)
	if code != 0 {
		t.Fatalf("status exit = %d, want 0; stderr=%q", code, statusErr.String())
	}
	if deps.routeCalls != 0 || deps.statusCalls != 1 {
		t.Fatalf("route calls = %d, status calls = %d; want status only", deps.routeCalls, deps.statusCalls)
	}
	if got := statusOut.String(); got != "{\"command\":\"status\",\"modes\":[],\"work_items\":[],\"evidence\":[]}\n" {
		t.Fatalf("status stdout = %q, want empty snapshot JSON", got)
	}
}

type countingDeps struct {
	runDeps
	routeCalls  int
	statusCalls int
}

func testDeps() *countingDeps {
	deps := &countingDeps{}
	deps.runDeps = runDeps{
		route: func(intent mode.Intent) mode.Decision {
			deps.routeCalls++
			return mode.Route(intent)
		},
		status: func(ctx context.Context, statePath string) (projection.Snapshot, error) {
			deps.statusCalls++
			return projection.Snapshot{
				Modes:     map[string]string{},
				WorkItems: map[string]projection.WorkItem{},
				Evidence:  map[string]projection.Evidence{},
			}, nil
		},
	}
	return deps
}

func assertStateUnavailable(t *testing.T, ctx context.Context, statePath string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(ctx, []string{"status", "--state", statePath}, &stdout, &stderr, productionDeps())
	if code != 3 {
		t.Fatalf("run status exit = %d, want 3; stderr=%q", code, stderr.String())
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if !strings.HasPrefix(stderr.String(), "state unavailable:") {
		t.Fatalf("stderr = %q, want state unavailable prefix", stderr.String())
	}
	if strings.Contains(stderr.String(), statePath) {
		t.Fatalf("stderr exposes state path %q: %q", statePath, stderr.String())
	}
}

func createJournalDB(t *testing.T) string {
	t.Helper()
	return createJournalDBNamed(t, "journal.db")
}

func createJournalDBNamed(t *testing.T, filename string) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), filename)
	db := openWritableDB(t, dbPath)
	defer db.Close()
	if err := journal.Migrate(context.Background(), db); err != nil {
		t.Fatalf("journal.Migrate() error = %v", err)
	}
	return dbPath
}

func createEmptySQLiteDB(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "empty.db")
	db := openWritableDB(t, dbPath)
	defer db.Close()
	if _, err := db.ExecContext(context.Background(), `CREATE TABLE unrelated (id TEXT PRIMARY KEY)`); err != nil {
		t.Fatalf("create unrelated table error = %v", err)
	}
	return dbPath
}

func writeProjectionFacts(t *testing.T, ctx context.Context, dbPath string) {
	t.Helper()
	db := openWritableDB(t, dbPath)
	defer db.Close()
	store := journal.NewStore(db)

	events := []journal.Event{
		journalEvent("evt-work-b", "work-stream-b", 1, "idem-work-b", "WorkItemCreated", map[string]string{"work_item_id": "work-b", "title": "Beta", "status": "open"}),
		journalEvent("evt-mode-b", "mode-b", 1, "idem-mode-b", "ModeSelected", map[string]string{"mode": "agent"}),
		journalEvent("evt-work-a", "work-stream-a", 1, "idem-work-a", "WorkItemCreated", map[string]string{"work_item_id": "work-a", "title": "Alpha", "status": "open"}),
		journalEvent("evt-evidence-b", "work-stream-b", 2, "idem-evidence-b", "EvidenceSubmitted", map[string]string{"evidence_id": "evidence-b", "work_item_id": "work-b", "digest": strings.Repeat("b", 64)}),
		journalEvent("evt-mode-a", "mode-a", 1, "idem-mode-a", "ModeSelected", map[string]string{"mode": "conversation"}),
		journalEvent("evt-evidence-a", "work-stream-a", 2, "idem-evidence-a", "EvidenceSubmitted", map[string]string{"evidence_id": "evidence-a", "work_item_id": "work-a", "digest": strings.Repeat("a", 64)}),
		journalEvent("evt-terminal-a", "work-stream-a", 3, "idem-terminal-a", "WorkItemTerminal", map[string]string{"work_item_id": "work-a", "status": "accepted"}),
	}
	for _, event := range events {
		if _, err := store.Append(ctx, event); err != nil {
			t.Fatalf("Append(%s) error = %v", event.ID, err)
		}
	}
}

func openWritableDB(t *testing.T, dbPath string) *sql.DB {
	t.Helper()
	values := url.Values{}
	values.Add("_pragma", "foreign_keys(1)")
	values.Add("_pragma", "busy_timeout(5000)")
	uri := url.URL{Scheme: "file", Path: dbPath}
	db, err := sql.Open("sqlite", fmt.Sprintf("%s?%s", uri.String(), values.Encode()))
	if err != nil {
		t.Fatalf("sql.Open() error = %v", err)
	}
	return db
}

func assertReadOnlyDBRejectsWrites(t *testing.T, ctx context.Context, dbPath string) {
	t.Helper()
	db, err := openReadOnlyState(ctx, dbPath)
	if err != nil {
		t.Fatalf("openReadOnlyState() error = %v", err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, `INSERT INTO events (id) VALUES ('blocked')`); err == nil {
		t.Fatal("read-only status database accepted write")
	}
}

func assertCLIDoesNotQueryJournalSQL(t *testing.T) {
	t.Helper()
	for _, path := range []string{"main.go", "query.go"} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s error = %v", path, err)
		}
		upper := strings.ToUpper(string(data))
		for _, forbidden := range []string{"SELECT ", "INSERT ", "UPDATE ", "DELETE "} {
			if strings.Contains(upper, forbidden) {
				t.Fatalf("cmd/loom/%s contains direct SQL verb %s", path, forbidden)
			}
		}
	}
}

func journalEvent(id, streamID string, seq int64, idempotencyKey string, eventType string, payload map[string]string) journal.Event {
	data, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return journal.Event{
		ID:             id,
		StreamID:       streamID,
		Seq:            seq,
		IdempotencyKey: idempotencyKey,
		Type:           eventType,
		SchemaVersion:  1,
		EmittedAt:      time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC),
		PayloadJSON:    data,
	}
}
