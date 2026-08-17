package roundtable

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"loom-pi-rebuild/internal/journal"
)

const exportCorrelation = "33333333-3333-4333-8333-333333333333"

// runConcludedJourney walks a session to conclusion and returns the view.
func runConcludedJourney(t *testing.T, authority *Authority, sessionID string) View {
	t.Helper()
	ctx := context.Background()
	view, err := authority.CreateSession(ctx, CreateSessionCommand{
		SessionID: sessionID, ModeratorSeat: "seat-moderator", Title: "Export journey",
		EmittedAt: utcTime(1), CorrelationID: exportCorrelation,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	for _, command := range []AddSeatCommand{
		{SessionID: sessionID, SeatID: "seat-writer", DisplayName: "Writer Seat", EmittedAt: utcTime(2), CorrelationID: exportCorrelation},
		{SessionID: sessionID, SeatID: "seat-target", DisplayName: "Target Seat", EmittedAt: utcTime(3), CorrelationID: exportCorrelation},
	} {
		if view, err = authority.AddSeat(ctx, command); err != nil {
			t.Fatalf("add seat: %v", err)
		}
	}
	if _, err := authority.OpenRound(ctx, OpenRoundCommand{
		SessionID: sessionID, RoundID: "round-1", ModeratorSeat: "seat-moderator",
		EmittedAt: utcTime(4), CorrelationID: exportCorrelation,
	}); err != nil {
		t.Fatalf("open round: %v", err)
	}
	if view, err = authority.ProposeMessage(ctx, ProposeMessageCommand{
		SessionID: sessionID, RoundID: "round-1", MessageID: "msg-1",
		WriterSeat: "seat-writer", TargetSeat: "seat-target",
		Body: "Export contract journey message.", ArtifactRefs: []string{},
		EmittedAt: utcTime(4), CorrelationID: exportCorrelation,
	}); err != nil {
		t.Fatalf("propose: %v", err)
	}
	if view, err = authority.RelayMessage(ctx, RelayMessageCommand{
		SessionID: sessionID, MessageID: "msg-1", ModeratorSeat: "seat-moderator",
		EmittedAt: utcTime(6), CorrelationID: exportCorrelation,
	}); err != nil {
		t.Fatalf("relay: %v", err)
	}
	if view, err = authority.AcknowledgeMessage(ctx, AcknowledgeMessageCommand{
		SessionID: sessionID, MessageID: "msg-1", SeatID: "seat-target",
		EmittedAt: utcTime(8), CorrelationID: exportCorrelation,
	}); err != nil {
		t.Fatalf("ack: %v", err)
	}
	if view, err = authority.InsertMessage(ctx, InsertMessageCommand{
		SessionID: sessionID, MessageID: "msg-1", ModeratorSeat: "seat-moderator",
		EmittedAt: utcTime(9), CorrelationID: exportCorrelation,
	}); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if view, err = authority.ConcludeSession(ctx, ConcludeSessionCommand{
		SessionID: sessionID, ModeratorSeat: "seat-moderator",
		EmittedAt: utcTime(10), CorrelationID: exportCorrelation,
	}); err != nil {
		t.Fatalf("conclude: %v", err)
	}
	if !view.Session.Concluded {
		t.Fatal("session not concluded")
	}
	return view
}

func journalConcluded(t *testing.T, journalStore *journal.Store, sessionID string) (summaryDigest string, concludedAt time.Time) {
	t.Helper()
	events, err := journalStore.ReadStream(context.Background(), sessionStream(sessionID))
	if err != nil {
		t.Fatal(err)
	}
	for index := len(events) - 1; index >= 0; index-- {
		if events[index].Type != FactConcluded {
			continue
		}
		var payload concludedPayload
		if err := decodeExact(events[index].PayloadJSON, &payload); err != nil {
			t.Fatal(err)
		}
		return payload.SummaryDigest, payload.ConcludedAt
	}
	t.Fatal("no concluded fact")
	return "", time.Time{}
}

func TestExportSessionPublishesContractArtifactAndFact(t *testing.T) {
	authority, evidenceStore, journalStore := newRoundtableFixture(t)
	ctx := context.Background()
	sessionID := "session-export-live"
	runConcludedJourney(t, authority, sessionID)
	summaryDigest, concludedAt := journalConcluded(t, journalStore, sessionID)
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(24 * time.Hour)

	result, err := authority.ExportSession(ctx, sessionID, expiresAt)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if result.SessionID != sessionID || result.Digest == "" ||
		!validSHA256Digest(result.Digest) ||
		!result.NotBefore.Equal(now) || !result.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("result = %#v", result)
	}
	// Artifact exists and hashes to the digest.
	artifact, err := evidenceStore.ReadArtifact(ctx, result.Digest, 1<<20)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	if digestBytes(string(artifact)) != result.Digest {
		t.Fatalf("artifact digest mismatch")
	}
	// Journal records the export fact with the same digest.
	events, err := journalStore.ReadStream(ctx, sessionStream(sessionID))
	if err != nil {
		t.Fatal(err)
	}
	last := events[len(events)-1]
	if last.Type != FactSessionExported {
		t.Fatalf("last fact = %s", last.Type)
	}
	var payload sessionExportedPayload
	if err := decodeExact(last.PayloadJSON, &payload); err != nil {
		t.Fatal(err)
	}
	if payload.SessionID != sessionID || payload.Digest != result.Digest ||
		payload.ExportID != result.ExportID {
		t.Fatalf("export fact payload = %#v", payload)
	}
	// Re-export yields a distinct export id/fact and the view is unchanged.
	again, err := authority.ExportSession(ctx, sessionID, expiresAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("re-export: %v", err)
	}
	if again.ExportID == result.ExportID {
		t.Fatal("re-export reused export id")
	}
	view, err := authority.ReadView(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if !view.Session.Concluded {
		t.Fatal("view regressed")
	}
	_ = summaryDigest
	_ = concludedAt
}

func TestExportSessionRejectsUnconcluded(t *testing.T) {
	authority, _, _ := newRoundtableFixture(t)
	ctx := context.Background()
	_, err := authority.CreateSession(ctx, CreateSessionCommand{
		SessionID: "session-export-open", ModeratorSeat: "seat-moderator",
		Title: "Open session", EmittedAt: utcTime(1), CorrelationID: exportCorrelation,
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)
	if _, err := authority.ExportSession(ctx, "session-export-open", now.Add(time.Hour)); err == nil {
		t.Fatal("unconcluded session exported")
	}
	if _, err := authority.ExportSession(ctx, "session-missing", now.Add(time.Hour)); !errors.Is(err, ErrRoundtableSessionNotFound) {
		t.Fatalf("missing session export = %v", err)
	}
}

func TestImportSessionIsIdempotentAndRejectsBad(t *testing.T) {
	authority, _, journalStore := newRoundtableFixture(t)
	ctx := context.Background()
	sessionID := "session-import-src"
	runConcludedJourney(t, authority, sessionID)
	summaryDigest, concludedAt := journalConcluded(t, journalStore, sessionID)
	now := time.Date(2026, 8, 16, 11, 0, 0, 0, time.UTC)
	view, err := authority.ReadView(ctx, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	contract, err := BuildExportContract(view, summaryDigest, concludedAt, now, now.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}

	first, err := authority.ImportSession(ctx, contract, exportCorrelation)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if !first.Recorded || first.View.Digest != contract.ViewDigest ||
		first.View.Session.ID != sessionID || !first.View.Session.Concluded {
		t.Fatalf("first import = %#v", first)
	}
	// Idempotent: re-import appends no second fact and returns the same view.
	second, err := authority.ImportSession(ctx, contract, exportCorrelation)
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	if second.Recorded || second.View.Digest != first.View.Digest {
		t.Fatalf("second import = %#v", second)
	}
	events, err := journalStore.ReadStream(ctx, sessionStream(sessionID))
	if err != nil {
		t.Fatal(err)
	}
	importFacts := 0
	for _, event := range events {
		if event.Type == FactSessionImported {
			importFacts++
		}
	}
	if importFacts != 1 {
		t.Fatalf("import facts = %d, want 1", importFacts)
	}

	// Expired contract rejected.
	expired := recontractDigest(t, func() ExportContract {
		mutated := contract
		mutated.ExpiresAt = now.Add(-time.Hour)
		return mutated
	}())
	if _, err := authority.ImportSession(ctx, expired, exportCorrelation); err == nil {
		t.Fatal("expired contract imported")
	}
	// Tampered contract rejected.
	tampered := recontractDigest(t, func() ExportContract {
		mutated := contract
		mutated.Summary.Rounds[0].Messages[0].Body = "tampered"
		return mutated
	}())
	if _, err := authority.ImportSession(ctx, tampered, exportCorrelation); err == nil {
		t.Fatal("tampered contract imported")
	}
	// Bad correlation rejected.
	if _, err := authority.ImportSession(ctx, contract, "not-a-uuid"); err == nil {
		t.Fatal("bad correlation accepted")
	}
}

func TestImportSessionUnknownSessionIsReadOnlyNoFact(t *testing.T) {
	authority, _, journalStore := newRoundtableFixture(t)
	ctx := context.Background()
	now := time.Date(2026, 8, 16, 11, 0, 0, 0, time.UTC)
	// Build a contract in one authority, then import it into a second,
	// empty authority where the session id does not exist.
	sourceID := "session-import-foreign-src"
	runConcludedJourney(t, authority, sourceID)
	summaryDigest, concludedAt := journalConcluded(t, journalStore, sourceID)
	view, err := authority.ReadView(ctx, sourceID)
	if err != nil {
		t.Fatal(err)
	}
	contract, err := BuildExportContract(view, summaryDigest, concludedAt, now, now.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	target, _, targetJournal := newRoundtableFixture(t)
	result, err := target.ImportSession(ctx, contract, exportCorrelation)
	if err != nil {
		t.Fatalf("import foreign: %v", err)
	}
	if result.Recorded {
		t.Fatal("foreign import recorded a fact")
	}
	if result.View.Digest != contract.ViewDigest ||
		strings.TrimSpace(result.View.Session.ID) == "" {
		t.Fatalf("reconstructed view = %#v", result.View)
	}
	events, err := targetJournal.ReadStream(ctx, sessionStream(sourceID))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("foreign import wrote facts: %#v", events)
	}
}
