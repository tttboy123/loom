package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/roundtable"
)

type productRoundtableFixture struct {
	authority   *roundtable.Authority
	controller  *productRoundtableController
	evidence    *evidence.Store
	journal     *journal.Store
	handler     func(context.Context, localipc.Request) localipc.Response
	concludedAt time.Time
}

func newProductRoundtableRouteFixture(t *testing.T) *productRoundtableFixture {
	t.Helper()
	realTemp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(realTemp, "roundtable-route")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	evidenceStore, err := evidence.NewStore(filepath.Join(root, "evidence"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = evidenceStore.Close() })
	database, err := sql.Open("sqlite", filepath.Join(root, "roundtable.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := journal.Migrate(context.Background(), database); err != nil {
		t.Fatal(err)
	}
	journalStore := journal.NewStore(database)
	now := func() time.Time {
		return time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	}
	authority, err := roundtable.NewAuthority(
		journalStore, evidenceStore, now, productRoundtableReader{},
	)
	if err != nil {
		t.Fatal(err)
	}
	controller, err := newProductRoundtableController(
		authority, now,
	)
	if err != nil {
		t.Fatal(err)
	}
	handler := newProductRouteHandler(productRouteServices{roundtable: controller})
	return &productRoundtableFixture{
		authority: authority, controller: controller, evidence: evidenceStore,
		journal: journalStore, handler: handler,
	}
}

type productRoundtableReader struct{}

func (productRoundtableReader) Read([]byte) (int, error) { return 0, io.EOF }

func (fixture *productRoundtableFixture) call(
	t *testing.T,
	method string,
	params any,
) localipc.Response {
	t.Helper()
	encoded, err := json.Marshal(params)
	if err != nil {
		t.Fatal(err)
	}
	return fixture.handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "roundtable-route-request", Method: method,
		Params: encoded,
	})
}

func (fixture *productRoundtableFixture) decodeView(
	t *testing.T,
	response localipc.Response,
) roundtable.View {
	t.Helper()
	if !response.OK || response.Error != nil {
		t.Fatalf("response = %#v", response)
	}
	var view roundtable.View
	if err := json.Unmarshal(response.Result, &view); err != nil {
		t.Fatalf("decode view: %v", err)
	}
	return view
}

const roundtableRouteCorrelation = "11111111-1111-4111-8111-111111111111"

func TestProductRoundtableRouteFullLifecycle(t *testing.T) {
	fixture := newProductRoundtableRouteFixture(t)
	params := productRoundtableSessionCreateParams{
		SchemaVersion: 1, SessionID: "session-route", ModeratorSeat: "seat-moderator",
		Title: "Governed diagnosis handoff", CorrelationID: roundtableRouteCorrelation,
	}
	view := fixture.decodeView(t, fixture.call(t, "roundtable_session_create", params))
	if view.Session.ModeratorSeat != "seat-moderator" || len(view.Seats) != 1 {
		t.Fatalf("session view = %#v", view)
	}
	view = fixture.decodeView(t, fixture.call(t, "roundtable_add_seat", productRoundtableAddSeatParams{
		SchemaVersion: 1, SessionID: "session-route", SeatID: "seat-writer",
		DisplayName: "Writer Seat", CorrelationID: roundtableRouteCorrelation,
	}))
	view = fixture.decodeView(t, fixture.call(t, "roundtable_add_seat", productRoundtableAddSeatParams{
		SchemaVersion: 1, SessionID: "session-route", SeatID: "seat-target",
		DisplayName: "Target Seat", CorrelationID: roundtableRouteCorrelation,
	}))
	if len(view.Seats) != 3 {
		t.Fatalf("seats = %#v", view.Seats)
	}
	fixture.decodeView(t, fixture.call(t, "roundtable_open_round", productRoundtableOpenRoundParams{
		SchemaVersion: 1, SessionID: "session-route", RoundID: "round-1",
		ModeratorSeat: "seat-moderator", CorrelationID: roundtableRouteCorrelation,
	}))
	view = fixture.decodeView(t, fixture.call(t, "roundtable_propose_message", productRoundtableProposeMessageParams{
		SchemaVersion: 1, SessionID: "session-route", RoundID: "round-1",
		MessageID: "msg-route", WriterSeat: "seat-writer", TargetSeat: "seat-target",
		Body: "Diagnosis: the route is over-constrained.", ArtifactRefs: []string{},
		CorrelationID: roundtableRouteCorrelation,
	}))
	if view.Messages["msg-route"].Status != roundtable.MessagePending {
		t.Fatalf("proposed status = %q", view.Messages["msg-route"].Status)
	}
	// Non-moderator relay is rejected with a typed code.
	rejected := fixture.call(t, "roundtable_relay_message", productRoundtableRelayMessageParams{
		SchemaVersion: 1, SessionID: "session-route", MessageID: "msg-route",
		ModeratorSeat: "seat-writer", CorrelationID: roundtableRouteCorrelation,
	})
	if rejected.OK || rejected.Error == nil || rejected.Error.Code != "not_moderator" {
		t.Fatalf("non-moderator relay code=%q msg=%q stage=%q recoverable=%t response=%#v",
			rejected.Error.Code, rejected.Error.Message, rejected.Error.Stage,
			rejected.Error.Recoverable, rejected)
	}
	view = fixture.decodeView(t, fixture.call(t, "roundtable_relay_message", productRoundtableRelayMessageParams{
		SchemaVersion: 1, SessionID: "session-route", MessageID: "msg-route",
		ModeratorSeat: "seat-moderator", CorrelationID: roundtableRouteCorrelation,
	}))
	if view.Messages["msg-route"].Status != roundtable.MessageRelayed {
		t.Fatalf("relayed status = %q", view.Messages["msg-route"].Status)
	}
	wrongAck := fixture.call(t, "roundtable_ack_message", productRoundtableAckMessageParams{
		SchemaVersion: 1, SessionID: "session-route", MessageID: "msg-route",
		SeatID: "seat-writer", CorrelationID: roundtableRouteCorrelation,
	})
	if wrongAck.OK || wrongAck.Error == nil || wrongAck.Error.Code != "not_found" {
		t.Fatalf("wrong-seat ack = %#v", wrongAck)
	}
	view = fixture.decodeView(t, fixture.call(t, "roundtable_ack_message", productRoundtableAckMessageParams{
		SchemaVersion: 1, SessionID: "session-route", MessageID: "msg-route",
		SeatID: "seat-target", CorrelationID: roundtableRouteCorrelation,
	}))
	if view.Messages["msg-route"].Status != roundtable.MessageAcknowledged {
		t.Fatalf("ack status = %q", view.Messages["msg-route"].Status)
	}
	view = fixture.decodeView(t, fixture.call(t, "roundtable_insert_message", productRoundtableInsertMessageParams{
		SchemaVersion: 1, SessionID: "session-route", MessageID: "msg-route",
		ModeratorSeat: "seat-moderator", CorrelationID: roundtableRouteCorrelation,
	}))
	if view.Messages["msg-route"].Status != roundtable.MessageInserted {
		t.Fatalf("insert status = %q", view.Messages["msg-route"].Status)
	}
	snapshot := fixture.decodeView(t, fixture.call(t, "roundtable_snapshot", productRoundtableSnapshotParams{
		SchemaVersion: 1, SessionID: "session-route",
	}))
	if snapshot.Digest != view.Digest ||
		snapshot.Messages["msg-route"].Status != roundtable.MessageInserted {
		t.Fatalf("snapshot mismatch digest=%q vs %q", snapshot.Digest, view.Digest)
	}
	concluded := fixture.decodeView(t, fixture.call(t, "roundtable_conclude", productRoundtableConcludeParams{
		SchemaVersion: 1, SessionID: "session-route",
		ModeratorSeat: "seat-moderator", CorrelationID: roundtableRouteCorrelation,
	}))
	if !concluded.Session.Concluded {
		t.Fatal("session not concluded")
	}
	// Post-conclude writes are rejected.
	after := fixture.call(t, "roundtable_propose_message", productRoundtableProposeMessageParams{
		SchemaVersion: 1, SessionID: "session-route", RoundID: "round-1",
		MessageID: "msg-after", WriterSeat: "seat-writer", TargetSeat: "seat-target",
		Body: "too late", ArtifactRefs: []string{},
		CorrelationID: roundtableRouteCorrelation,
	})
	if after.OK || after.Error == nil || after.Error.Code != "concluded" {
		t.Fatalf("post-conclude write = %#v", after)
	}
	// Strict decode: unknown field and bad schema version are invalid_request.
	invalid := fixture.call(t, "roundtable_session_create", map[string]any{
		"schema_version": 1, "session_id": "session-bad",
		"moderator_seat": "seat-moderator", "title": "bad",
		"correlation_id": roundtableRouteCorrelation, "extra": true,
	})
	if invalid.OK || invalid.Error == nil || invalid.Error.Code != "invalid_request" {
		t.Fatalf("unknown-field response = %#v", invalid)
	}
	badVersion := fixture.call(t, "roundtable_snapshot", productRoundtableSnapshotParams{
		SchemaVersion: 2, SessionID: "session-route",
	})
	if badVersion.OK || badVersion.Error == nil || badVersion.Error.Code != "invalid_request" {
		t.Fatalf("bad version response = %#v", badVersion)
	}
	missing := fixture.call(t, "roundtable_snapshot", productRoundtableSnapshotParams{
		SchemaVersion: 1, SessionID: "session-missing",
	})
	if missing.OK || missing.Error == nil || missing.Error.Code != "not_found" {
		t.Fatalf("missing session response = %#v", missing)
	}
}

func TestProductRoundtableRouteRejoinsRetiredSeatBeforeRound(t *testing.T) {
	fixture := newProductRoundtableRouteFixture(t)
	const sessionID = "session-route-rejoin"
	const seatID = "agent-reviewer"

	fixture.decodeView(t, fixture.call(t, "roundtable_session_create", productRoundtableSessionCreateParams{
		SchemaVersion: 1, SessionID: sessionID, ModeratorSeat: "seat-moderator",
		Title: "Rejoin Agent", CorrelationID: roundtableRouteCorrelation,
	}))
	add := productRoundtableAddSeatParams{
		SchemaVersion: 1, SessionID: sessionID, SeatID: seatID,
		DisplayName: "Reviewer", CorrelationID: roundtableRouteCorrelation,
	}
	fixture.decodeView(t, fixture.call(t, "roundtable_add_seat", add))
	retired := fixture.decodeView(t, fixture.call(t, "roundtable_retire_seat", productRoundtableRetireSeatParams{
		SchemaVersion: 1, SessionID: sessionID, SeatID: seatID,
		ModeratorSeat: "seat-moderator", CorrelationID: roundtableRouteCorrelation,
	}))
	if retired.Seats[seatID].Available {
		t.Fatalf("retired seat remained active: %#v", retired.Seats[seatID])
	}
	rejoined := fixture.decodeView(t, fixture.call(t, "roundtable_add_seat", add))
	if !rejoined.Seats[seatID].Available || len(rejoined.Rounds) != 0 {
		t.Fatalf("rejoined view = %#v", rejoined)
	}
	// Add is shared by the workbench button and drop destination. A retry must
	// return the same authoritative view without appending another fact.
	retried := fixture.decodeView(t, fixture.call(t, "roundtable_add_seat", add))
	if retried.Digest != rejoined.Digest {
		t.Fatalf("retry digest = %q, want %q", retried.Digest, rejoined.Digest)
	}
	events, err := fixture.journal.ReadStream(context.Background(), "roundtable/session/"+sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 || events[3].Type != roundtable.FactSeatRejoined {
		t.Fatalf("route facts = %#v", events)
	}
}

func TestProductRoundtableRouteNilServiceIsStateUnavailable(t *testing.T) {
	handler := newProductRouteHandler(productRouteServices{})
	params, err := json.Marshal(productRoundtableSnapshotParams{
		SchemaVersion: 1, SessionID: "session-any",
	})
	if err != nil {
		t.Fatal(err)
	}
	response := handler(context.Background(), localipc.Request{
		Version: 1, RequestID: "roundtable-nil-service", Method: "roundtable_snapshot",
		Params: params,
	})
	if response.OK || response.Error == nil || response.Error.Code != "state_unavailable" {
		t.Fatalf("nil service response = %#v", response)
	}
}

func TestProductRoundtableRouteTraversesAuthenticatedLocalIPC(t *testing.T) {
	fixture := newProductRoundtableRouteFixture(t)
	root, err := os.MkdirTemp("/private/tmp", "loom-roundtable-route-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if err := os.Chmod(root, 0o700); err != nil {
		t.Fatal(err)
	}
	server, err := localipc.NewServer(localipc.ServerConfig{
		SocketPath: filepath.Join(root, "loomd.sock"), EffectiveUID: os.Geteuid(),
		BuildID: "roundtable-route-ipc-fixture",
		Handler: localipc.HandlerFunc(fixture.handler),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- server.Serve(ctx) }()
	select {
	case <-server.Ready():
	case err := <-done:
		t.Fatalf("server failed before ready: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("server did not become ready")
	}
	defer func() {
		cancel()
		if err := <-done; err != nil && !errors.Is(err, context.Canceled) {
			t.Errorf("server close: %v", err)
		}
	}()
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: filepath.Join(root, "loomd.sock"), Timeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	var created roundtable.View
	if err := client.Call(
		context.Background(), "roundtable_session_create",
		productRoundtableSessionCreateParams{
			SchemaVersion: 1, SessionID: "session-ipc", ModeratorSeat: "seat-moderator",
			Title: "IPC handoff", CorrelationID: roundtableRouteCorrelation,
		},
		&created,
	); err != nil {
		t.Fatal(err)
	}
	if created.Session.ModeratorSeat != "seat-moderator" || len(created.Seats) != 1 {
		t.Fatalf("created view = %#v", created)
	}
	var concluded roundtable.View
	if err := client.Call(
		context.Background(), "roundtable_conclude",
		productRoundtableConcludeParams{
			SchemaVersion: 1, SessionID: "session-ipc",
			ModeratorSeat: "seat-moderator", CorrelationID: roundtableRouteCorrelation,
		},
		&concluded,
	); err != nil {
		t.Fatal(err)
	}
	if !concluded.Session.Concluded || len(concluded.Seats) != 1 {
		t.Fatalf("concluded view = %#v", concluded)
	}
}

func mustProductRoundtableJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
