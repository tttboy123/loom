package roundtable

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"loom-pi-rebuild/internal/evidence"
	"loom-pi-rebuild/internal/journal"
)

func newRoundtableFixture(t *testing.T) (*Authority, *evidence.Store, *journal.Store) {
	t.Helper()
	realTemp, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(realTemp, "roundtable")
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
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
	now := func() time.Time { return time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC) }
	authority, err := NewAuthority(
		journalStore, evidenceStore, now, deterministicReader{},
	)
	if err != nil {
		t.Fatal(err)
	}
	return authority, evidenceStore, journalStore
}

type deterministicReader struct{}

func (deterministicReader) Read([]byte) (int, error) { return 0, io.EOF }

func utcTime(second int) time.Time {
	return time.Date(2026, 8, 16, 12, 0, second, 0, time.UTC)
}

func TestRoundtableFullLifecycleAndReplay(t *testing.T) {
	authority, evidenceStore, journalStore := newRoundtableFixture(t)
	ctx := context.Background()
	sessionID := "session-alpha"
	moderator := "seat-moderator"
	writer := "seat-writer"
	target := "seat-target"
	messageID := "msg-1"
	correlation := "11111111-1111-4111-8111-111111111111"

	view, err := authority.CreateSession(ctx, CreateSessionCommand{
		SessionID: sessionID, ModeratorSeat: moderator, Title: "Diagnosis handoff",
		EmittedAt: utcTime(1), CorrelationID: correlation,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if view.Session.ModeratorSeat != moderator ||
		view.Session.Title != "Diagnosis handoff" || len(view.Seats) != 1 {
		t.Fatalf("session view = %#v", view)
	}
	for _, command := range []AddSeatCommand{
		{SessionID: sessionID, SeatID: writer, DisplayName: "Writer Seat", EmittedAt: utcTime(2), CorrelationID: correlation},
		{SessionID: sessionID, SeatID: target, DisplayName: "Target Seat", EmittedAt: utcTime(3), CorrelationID: correlation},
	} {
		view, err = authority.AddSeat(ctx, command)
		if err != nil {
			t.Fatalf("add seat: %v", err)
		}
	}
	if len(view.Seats) != 3 {
		t.Fatalf("seats = %#v", view.Seats)
	}
	if _, err := authority.OpenRound(ctx, OpenRoundCommand{
		SessionID: sessionID, RoundID: "round-1", ModeratorSeat: moderator,
		EmittedAt: utcTime(4), CorrelationID: correlation,
	}); err != nil {
		t.Fatalf("open round: %v", err)
	}

	// Propose a bounded message (pending).
	body := "Diagnosis: the daemon build_execution path is over-constrained."
	view, err = authority.ProposeMessage(ctx, ProposeMessageCommand{
		SessionID: sessionID, RoundID: "round-1", MessageID: messageID,
		WriterSeat: writer, TargetSeat: target, Body: body,
		ArtifactRefs: []string{strings.Repeat("a", 64)},
		EmittedAt:    utcTime(4), CorrelationID: correlation,
	})
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if view.Messages[messageID].Status != MessagePending {
		t.Fatalf("message status = %q", view.Messages[messageID].Status)
	}
	expectedDigest := digestBytes(sessionID, messageID, body)
	if view.Messages[messageID].BodyDigest != expectedDigest {
		t.Fatalf("digest = %q want %q", view.Messages[messageID].BodyDigest, expectedDigest)
	}

	// Non-moderator relay is rejected.
	if _, err := authority.RelayMessage(ctx, RelayMessageCommand{
		SessionID: sessionID, MessageID: messageID, ModeratorSeat: writer,
		EmittedAt: utcTime(5), CorrelationID: correlation,
	}); !errors.Is(err, ErrRoundtableNotModerator) {
		t.Fatalf("non-moderator relay = %v", err)
	}
	// Moderator relay.
	view, err = authority.RelayMessage(ctx, RelayMessageCommand{
		SessionID: sessionID, MessageID: messageID, ModeratorSeat: moderator,
		EmittedAt: utcTime(6), CorrelationID: correlation,
	})
	if err != nil {
		t.Fatalf("relay: %v", err)
	}
	if view.Messages[messageID].Status != MessageRelayed {
		t.Fatalf("relay status = %q", view.Messages[messageID].Status)
	}
	// Wrong seat cannot ack.
	if _, err := authority.AcknowledgeMessage(ctx, AcknowledgeMessageCommand{
		SessionID: sessionID, MessageID: messageID, SeatID: writer,
		EmittedAt: utcTime(7), CorrelationID: correlation,
	}); !errors.Is(err, ErrRoundtableSeatNotFound) {
		t.Fatalf("wrong-seat ack = %v", err)
	}
	// Target acks (per-hop confirmation).
	view, err = authority.AcknowledgeMessage(ctx, AcknowledgeMessageCommand{
		SessionID: sessionID, MessageID: messageID, SeatID: target,
		EmittedAt: utcTime(8), CorrelationID: correlation,
	})
	if err != nil {
		t.Fatalf("ack: %v", err)
	}
	if view.Messages[messageID].Status != MessageAcknowledged {
		t.Fatalf("ack status = %q", view.Messages[messageID].Status)
	}
	// Moderator inserts.
	view, err = authority.InsertMessage(ctx, InsertMessageCommand{
		SessionID: sessionID, MessageID: messageID, ModeratorSeat: moderator,
		EmittedAt: utcTime(9), CorrelationID: correlation,
	})
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if view.Messages[messageID].Status != MessageInserted {
		t.Fatalf("insert status = %q", view.Messages[messageID].Status)
	}

	// Conclude: summary Artifact + digest recorded.
	view, err = authority.ConcludeSession(ctx, ConcludeSessionCommand{
		SessionID: sessionID, ModeratorSeat: moderator,
		EmittedAt: utcTime(10), CorrelationID: correlation,
	})
	if err != nil {
		t.Fatalf("conclude: %v", err)
	}
	if !view.Session.Concluded {
		t.Fatal("session not concluded")
	}
	concludedEvents, err := journalStore.ReadStream(ctx, sessionStream(sessionID))
	if err != nil {
		t.Fatal(err)
	}
	if len(concludedEvents) == 0 || concludedEvents[len(concludedEvents)-1].Type != FactConcluded {
		t.Fatalf("concluded fact missing: %#v", concludedEvents)
	}
	var concludedPayloadValue concludedPayload
	if err := decodeExact(concludedEvents[len(concludedEvents)-1].PayloadJSON, &concludedPayloadValue); err != nil {
		t.Fatal(err)
	}
	artifactBytes, err := evidenceStore.ReadArtifact(
		ctx, concludedPayloadValue.SummaryDigest, 1<<20,
	)
	if err != nil {
		t.Fatalf("summary artifact: %v", err)
	}
	if digestBytes(string(artifactBytes)) != concludedPayloadValue.SummaryDigest {
		t.Fatalf("artifact bytes do not hash to the recorded digest")
	}
	var summary AlignmentSummary
	if err := decodeExact(artifactBytes, &summary); err != nil ||
		summary.SessionID != sessionID || len(summary.Seats) != 3 ||
		len(summary.Rounds) != 1 {
		t.Fatalf("summary artifact = %#v, %v", summary, err)
	}

	// After conclude, writes are rejected.
	if _, err := authority.ProposeMessage(ctx, ProposeMessageCommand{
		SessionID: sessionID, RoundID: "round-1", MessageID: "msg-2",
		WriterSeat: writer, TargetSeat: target, Body: "late",
		EmittedAt: utcTime(11), CorrelationID: correlation,
	}); !errors.Is(err, ErrRoundtableAlreadyConcluded) {
		t.Fatalf("post-conclude propose = %v", err)
	}

	// Replay from the Journal reproduces the same view.
	replayed, err := authority.ReadView(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if !replayed.Session.Concluded || len(replayed.Seats) != 3 ||
		replayed.Messages[messageID].Status != MessageInserted {
		t.Fatalf("replayed view = %#v", replayed)
	}
}

func TestRoundtableBoundedBodyAndDigestOnlyArtifactRefs(t *testing.T) {
	authority, _, _ := newRoundtableFixture(t)
	ctx := context.Background()
	sessionID := "session-bounds"
	moderator := "seat-moderator"
	writer := "seat-writer"
	target := "seat-target"
	correlation := "11111111-1111-4111-8111-111111111111"
	if _, err := authority.CreateSession(ctx, CreateSessionCommand{
		SessionID: sessionID, ModeratorSeat: moderator, Title: "Bounds",
		EmittedAt: utcTime(1), CorrelationID: correlation,
	}); err != nil {
		t.Fatal(err)
	}
	authority.AddSeat(ctx, AddSeatCommand{SessionID: sessionID, SeatID: writer, DisplayName: "W", EmittedAt: utcTime(2), CorrelationID: correlation})
	authority.AddSeat(ctx, AddSeatCommand{SessionID: sessionID, SeatID: target, DisplayName: "T", EmittedAt: utcTime(3), CorrelationID: correlation})
	if _, err := authority.OpenRound(ctx, OpenRoundCommand{
		SessionID: sessionID, RoundID: "round-1", ModeratorSeat: moderator,
		EmittedAt: utcTime(4), CorrelationID: correlation,
	}); err != nil {
		t.Fatal(err)
	}

	// Body > 8 KiB rejected.
	oversized := strings.Repeat("x", MaxMessageBodyBytes+1)
	if _, err := authority.ProposeMessage(ctx, ProposeMessageCommand{
		SessionID: sessionID, RoundID: "round-1", MessageID: "msg-big",
		WriterSeat: writer, TargetSeat: target, Body: oversized,
		EmittedAt: utcTime(4), CorrelationID: correlation,
	}); !errors.Is(err, ErrRoundtableInvalidBody) {
		t.Fatalf("oversized body = %v", err)
	}
	// Non-digest artifact ref rejected.
	if _, err := authority.ProposeMessage(ctx, ProposeMessageCommand{
		SessionID: sessionID, RoundID: "round-1", MessageID: "msg-badref",
		WriterSeat: writer, TargetSeat: target, Body: "ok",
		ArtifactRefs: []string{"not-a-digest"},
		EmittedAt:    utcTime(5), CorrelationID: correlation,
	}); !errors.Is(err, ErrInvalidRoundtableMessage) {
		t.Fatalf("bad artifact ref = %v", err)
	}
	// Unknown round rejected.
	if _, err := authority.ProposeMessage(ctx, ProposeMessageCommand{
		SessionID: sessionID, RoundID: "round-unknown", MessageID: "msg-3",
		WriterSeat: writer, TargetSeat: target, Body: "ok",
		EmittedAt: utcTime(6), CorrelationID: correlation,
	}); !errors.Is(err, ErrRoundtableRoundNotFound) {
		t.Fatalf("unknown round = %v", err)
	}
}

func TestRoundtableSeatRetiredRejectsWrites(t *testing.T) {
	authority, _, _ := newRoundtableFixture(t)
	ctx := context.Background()
	sessionID := "session-retire"
	moderator := "seat-moderator"
	writer := "seat-writer"
	target := "seat-target"
	correlation := "11111111-1111-4111-8111-111111111111"
	authority.CreateSession(ctx, CreateSessionCommand{SessionID: sessionID, ModeratorSeat: moderator, Title: "Retire", EmittedAt: utcTime(1), CorrelationID: correlation})
	authority.AddSeat(ctx, AddSeatCommand{SessionID: sessionID, SeatID: writer, DisplayName: "W", EmittedAt: utcTime(2), CorrelationID: correlation})
	authority.AddSeat(ctx, AddSeatCommand{SessionID: sessionID, SeatID: target, DisplayName: "T", EmittedAt: utcTime(3), CorrelationID: correlation})
	if _, err := authority.OpenRound(ctx, OpenRoundCommand{
		SessionID: sessionID, RoundID: "round-1", ModeratorSeat: moderator,
		EmittedAt: utcTime(4), CorrelationID: correlation,
	}); err != nil {
		t.Fatal(err)
	}
	view, err := authority.RetireSeat(ctx, RetireSeatCommand{
		SessionID: sessionID, SeatID: writer, ModeratorSeat: moderator,
		EmittedAt: utcTime(5), CorrelationID: correlation,
	})
	if err != nil {
		t.Fatalf("retire: %v", err)
	}
	if view.Seats[writer].Available {
		t.Fatal("writer seat still available")
	}
	if _, err := authority.ProposeMessage(ctx, ProposeMessageCommand{
		SessionID: sessionID, RoundID: "round-1", MessageID: "msg-1",
		WriterSeat: writer, TargetSeat: target, Body: "ok",
		EmittedAt: utcTime(5), CorrelationID: correlation,
	}); !errors.Is(err, ErrRoundtableSeatUnavailable) {
		t.Fatalf("retired seat write = %v", err)
	}
}

func TestRoundtableRetiredSeatCanRejoinBeforeRoundWithReplayAndIdempotency(t *testing.T) {
	authority, _, journalStore := newRoundtableFixture(t)
	ctx := context.Background()
	const sessionID = "session-rejoin"
	const moderator = "seat-moderator"
	const seatID = "agent-reviewer"
	const correlation = "11111111-1111-4111-8111-111111111111"

	if _, err := authority.CreateSession(ctx, CreateSessionCommand{
		SessionID: sessionID, ModeratorSeat: moderator, Title: "Rejoin",
		EmittedAt: utcTime(1), CorrelationID: correlation,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.AddSeat(ctx, AddSeatCommand{
		SessionID: sessionID, SeatID: seatID, DisplayName: "Reviewer",
		EmittedAt: utcTime(2), CorrelationID: correlation,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := authority.RetireSeat(ctx, RetireSeatCommand{
		SessionID: sessionID, SeatID: seatID, ModeratorSeat: moderator,
		EmittedAt: utcTime(3), CorrelationID: correlation,
	}); err != nil {
		t.Fatal(err)
	}
	rejoin := AddSeatCommand{
		SessionID: sessionID, SeatID: seatID, DisplayName: "Reviewer",
		EmittedAt: utcTime(4), CorrelationID: correlation,
	}
	first, err := authority.AddSeat(ctx, rejoin)
	if err != nil {
		t.Fatalf("rejoin retired seat: %v", err)
	}
	if !first.Seats[seatID].Available || len(first.Rounds) != 0 {
		t.Fatalf("rejoined view = %#v", first)
	}
	second, err := authority.AddSeat(ctx, rejoin)
	if err != nil {
		t.Fatalf("idempotent rejoin: %v", err)
	}
	if second.Digest != first.Digest {
		t.Fatalf("idempotent rejoin digest = %q, want %q", second.Digest, first.Digest)
	}

	events, err := journalStore.ReadStream(ctx, sessionStream(sessionID))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 4 || events[3].Type != FactSeatRejoined {
		t.Fatalf("rejoin facts = %#v", events)
	}
	var payload seatRejoinedPayload
	if err := decodeExact(events[3].PayloadJSON, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.SeatID != seatID || payload.MembershipRevision != 2 {
		t.Fatalf("rejoin payload = %#v", payload)
	}

	replayed, err := authority.ReadView(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Digest != first.Digest || !replayed.Seats[seatID].Available {
		t.Fatalf("replayed view = %#v, want digest %q", replayed, first.Digest)
	}

	if _, err := authority.RetireSeat(ctx, RetireSeatCommand{
		SessionID: sessionID, SeatID: seatID, ModeratorSeat: moderator,
		EmittedAt: utcTime(5), CorrelationID: correlation,
	}); err != nil {
		t.Fatalf("retire rejoined seat: %v", err)
	}
	third, err := authority.AddSeat(ctx, AddSeatCommand{
		SessionID: sessionID, SeatID: seatID, DisplayName: "Reviewer",
		EmittedAt: utcTime(6), CorrelationID: correlation,
	})
	if err != nil {
		t.Fatalf("rejoin seat a second time: %v", err)
	}
	events, err = journalStore.ReadStream(ctx, sessionStream(sessionID))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 6 || events[4].Type != FactSeatRetired ||
		events[5].Type != FactSeatRejoined {
		t.Fatalf("second membership facts = %#v", events)
	}
	if err := decodeExact(events[5].PayloadJSON, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.MembershipRevision != 3 {
		t.Fatalf("second rejoin payload = %#v", payload)
	}
	replayed, err = authority.ReadView(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if replayed.Digest != third.Digest || !replayed.Seats[seatID].Available {
		t.Fatalf("second replayed view = %#v, want digest %q", replayed, third.Digest)
	}
}

func TestRoundtableProposeIdempotentOnExactFact(t *testing.T) {
	authority, _, _ := newRoundtableFixture(t)
	ctx := context.Background()
	sessionID := "session-idem"
	moderator := "seat-moderator"
	writer := "seat-writer"
	target := "seat-target"
	correlation := "11111111-1111-4111-8111-111111111111"
	authority.CreateSession(ctx, CreateSessionCommand{SessionID: sessionID, ModeratorSeat: moderator, Title: "Idem", EmittedAt: utcTime(1), CorrelationID: correlation})
	authority.AddSeat(ctx, AddSeatCommand{SessionID: sessionID, SeatID: writer, DisplayName: "W", EmittedAt: utcTime(2), CorrelationID: correlation})
	authority.AddSeat(ctx, AddSeatCommand{SessionID: sessionID, SeatID: target, DisplayName: "T", EmittedAt: utcTime(3), CorrelationID: correlation})
	if _, err := authority.OpenRound(ctx, OpenRoundCommand{
		SessionID: sessionID, RoundID: "round-1", ModeratorSeat: moderator,
		EmittedAt: utcTime(4), CorrelationID: correlation,
	}); err != nil {
		t.Fatal(err)
	}
	propose := ProposeMessageCommand{
		SessionID: sessionID, RoundID: "round-1", MessageID: "msg-1",
		WriterSeat: writer, TargetSeat: target, Body: "same body",
		EmittedAt: utcTime(4), CorrelationID: correlation,
	}
	first, err := authority.ProposeMessage(ctx, propose)
	if err != nil {
		t.Fatal(err)
	}
	second, err := authority.ProposeMessage(ctx, propose)
	if err != nil {
		t.Fatalf("idempotent re-propose: %v", err)
	}
	if first.Digest != second.Digest ||
		second.Messages["msg-1"].BodyDigest != first.Messages["msg-1"].BodyDigest {
		t.Fatalf("idempotency drift: %#v vs %#v", first, second)
	}
	// Same message ID with a DIFFERENT body conflicts.
	propose.Body = "different body"
	if _, err := authority.ProposeMessage(ctx, propose); !errors.Is(err, ErrRoundtableConflict) {
		t.Fatalf("conflicting re-propose = %v", err)
	}
}

func TestRoundtableConcludeSummaryIsCanonicalAndBounded(t *testing.T) {
	authority, evidenceStore, _ := newRoundtableFixture(t)
	ctx := context.Background()
	sessionID := "session-summary"
	moderator := "seat-moderator"
	writer := "seat-writer"
	target := "seat-target"
	correlation := "11111111-1111-4111-8111-111111111111"
	authority.CreateSession(ctx, CreateSessionCommand{SessionID: sessionID, ModeratorSeat: moderator, Title: "Summary", EmittedAt: utcTime(1), CorrelationID: correlation})
	authority.AddSeat(ctx, AddSeatCommand{SessionID: sessionID, SeatID: writer, DisplayName: "Writer Seat", EmittedAt: utcTime(2), CorrelationID: correlation})
	authority.AddSeat(ctx, AddSeatCommand{SessionID: sessionID, SeatID: target, DisplayName: "Target Seat", EmittedAt: utcTime(3), CorrelationID: correlation})
	authority.ProposeMessage(ctx, ProposeMessageCommand{
		SessionID: sessionID, RoundID: "round-1", MessageID: "msg-1",
		WriterSeat: writer, TargetSeat: target,
		Body:         "Findings: the adapter limit is the root cause.",
		ArtifactRefs: []string{strings.Repeat("b", 64)},
		EmittedAt:    utcTime(4), CorrelationID: correlation,
	})
	authority.RelayMessage(ctx, RelayMessageCommand{SessionID: sessionID, MessageID: "msg-1", ModeratorSeat: moderator, EmittedAt: utcTime(5), CorrelationID: correlation})
	authority.AcknowledgeMessage(ctx, AcknowledgeMessageCommand{SessionID: sessionID, MessageID: "msg-1", SeatID: target, EmittedAt: utcTime(6), CorrelationID: correlation})
	view, err := authority.ConcludeSession(ctx, ConcludeSessionCommand{
		SessionID: sessionID, ModeratorSeat: moderator,
		EmittedAt: utcTime(7), CorrelationID: correlation,
	})
	if err != nil {
		t.Fatalf("conclude: %v", err)
	}
	// Locate the concluded fact digest.
	events, err := authority.store.ReadStream(ctx, sessionStream(sessionID))
	if err != nil {
		t.Fatal(err)
	}
	var concludedPayloadValue concludedPayload
	for _, event := range events {
		if event.Type == FactConcluded {
			_ = decodeExact(event.PayloadJSON, &concludedPayloadValue)
		}
	}
	if concludedPayloadValue.SummaryDigest == "" {
		t.Fatal("no summary digest")
	}
	summaryArtifact, err := evidenceStore.ReadArtifact(
		ctx, concludedPayloadValue.SummaryDigest, 1<<20,
	)
	if err != nil {
		t.Fatal(err)
	}
	if digestBytes(string(summaryArtifact)) != concludedPayloadValue.SummaryDigest {
		t.Fatalf("artifact digest mismatch")
	}
	_ = view
}
