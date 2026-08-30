package roundtable

import (
	"strings"
	"testing"
	"time"
)

func concludedViewFixture() View {
	view := View{
		Session: Session{
			ID: "session-export", ModeratorSeat: "seat-moderator",
			Title: "Export handoff", CreatedAt: time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC),
			Concluded: true,
		},
		Seats: map[string]Seat{
			"seat-moderator": {ID: "seat-moderator", DisplayName: "Moderator", Available: true},
			"seat-writer":    {ID: "seat-writer", DisplayName: "Writer Seat", Available: true},
			"seat-target":    {ID: "seat-target", DisplayName: "Target Seat", Available: true},
		},
		Rounds: []Round{{
			ID: "round-1", Sequence: 1, MessageCount: 1,
			Messages: []Message{{
				ID: "msg-1", RoundID: "round-1",
				WriterSeat: "seat-writer", TargetSeat: "seat-target",
				Body:         "Governed handoff: bounded export contract message.",
				ArtifactRefs: []string{},
				BodyDigest:   strings.Repeat("b", 64),
				Status:       MessageInserted,
			}},
		}},
		Messages: map[string]Message{
			"msg-1": {
				ID: "msg-1", RoundID: "round-1",
				WriterSeat: "seat-writer", TargetSeat: "seat-target",
				Body:         "Governed handoff: bounded export contract message.",
				ArtifactRefs: []string{},
				BodyDigest:   strings.Repeat("b", 64),
				Status:       MessageInserted,
			},
		},
		Digest: strings.Repeat("a", 64),
	}
	return view
}

func TestExportContractBuildIsCanonicalAndDigestBound(t *testing.T) {
	view := concludedViewFixture()
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(24 * time.Hour)
	summary := BuildAlignmentSummary(view, now)
	summaryDigest := digestBytes(string(mustMarshalJSON(t, summary)))

	contract, err := BuildExportContract(view, summaryDigest, now, now, expiresAt)
	if err != nil {
		t.Fatalf("build export contract: %v", err)
	}
	if contract.SchemaVersion != ExportSchemaVersion ||
		contract.SessionID != "session-export" ||
		contract.ModeratorSeat != "seat-moderator" ||
		!contract.ConcludedAt.Equal(now) ||
		!contract.NotBefore.Equal(now) ||
		!contract.ExpiresAt.Equal(expiresAt) ||
		contract.SummaryDigest != summaryDigest ||
		contract.ViewDigest != view.Digest ||
		!validSHA256Digest(contract.Digest) {
		t.Fatalf("contract = %#v", contract)
	}
	// Canonical: marshaling twice yields identical bytes and digest.
	first, err := marshalContract(contract)
	if err != nil {
		t.Fatal(err)
	}
	second, err := marshalContract(contract)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Fatalf("contract not canonical: %s vs %s", first, second)
	}
	// The digest covers the digest-less canonical form; import validates it.
	if err := ValidateImportContract(contract, now); err != nil {
		t.Fatalf("freshly built contract fails import: %v", err)
	}
}

// recontractDigest recomputes the contract digest after a mutation (digest
// covers the canonical JSON without the Digest field).
func recontractDigest(t *testing.T, contract ExportContract) ExportContract {
	t.Helper()
	recomputed := contract
	recomputed.Digest = ""
	recomputed.Digest = digestBytes(string(mustMarshalJSON(t, recomputed)))
	return recomputed
}

func TestExportContractValidateImportRejectsExpiredTamperedForeign(t *testing.T) {
	view := concludedViewFixture()
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	expiresAt := now.Add(24 * time.Hour)
	summary := BuildAlignmentSummary(view, now)
	summaryDigest := digestBytes(string(mustMarshalJSON(t, summary)))
	valid, err := BuildExportContract(view, summaryDigest, now, now, expiresAt)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateImportContract(valid, now.Add(1*time.Hour)); err != nil {
		t.Fatalf("valid contract rejected: %v", err)
	}

	// Expired: build a valid contract then move ExpiresAt into the past and
	// recompute the digest (the builder itself refuses a non-future expiry).
	expired := recontractDigest(t, func() ExportContract {
		mutated := valid
		mutated.ExpiresAt = now.Add(-1 * time.Hour)
		return mutated
	}())
	if err := ValidateImportContract(expired, now); err == nil {
		t.Fatal("expired contract accepted")
	}

	// Not yet valid.
	notYet := recontractDigest(t, func() ExportContract {
		mutated := valid
		mutated.NotBefore = now.Add(2 * time.Hour)
		return mutated
	}())
	if err := ValidateImportContract(notYet, now); err == nil {
		t.Fatal("not-yet-valid contract accepted")
	}

	// Tampered summary: change a message body after digesting.
	tampered := valid
	tampered.Summary.Rounds[0].Messages[0].Body = "tampered body"
	tampered = recontractDigest(t, tampered)
	if err := ValidateImportContract(tampered, now); err == nil {
		t.Fatal("tampered summary accepted")
	}

	// Wrong schema version.
	badSchema := valid
	badSchema.SchemaVersion = 2
	badSchema = recontractDigest(t, badSchema)
	if err := ValidateImportContract(badSchema, now); err == nil {
		t.Fatal("bad schema accepted")
	}

	// Foreign / malformed provenance: session id missing.
	badSession := valid
	badSession.SessionID = ""
	badSession = recontractDigest(t, badSession)
	if err := ValidateImportContract(badSession, now); err == nil {
		t.Fatal("malformed session accepted")
	}

	for name, mutate := range map[string]func(ExportContract) ExportContract{
		"session identity": func(contract ExportContract) ExportContract {
			contract.SessionID = "session-different"
			return contract
		},
		"moderator identity": func(contract ExportContract) ExportContract {
			contract.ModeratorSeat = "seat-different"
			return contract
		},
		"conclusion time": func(contract ExportContract) ExportContract {
			contract.ConcludedAt = contract.ConcludedAt.Add(time.Second)
			return contract
		},
	} {
		t.Run("mismatched "+name, func(t *testing.T) {
			mismatched := recontractDigest(t, mutate(valid))
			if err := ValidateImportContract(mismatched, now); err == nil {
				t.Fatalf("mismatched %s accepted", name)
			}
		})
	}
}

func TestExportContractBuildRejectsUnconcludedOrBadDigest(t *testing.T) {
	view := concludedViewFixture()
	now := time.Date(2026, 8, 17, 12, 0, 0, 0, time.UTC)
	summary := BuildAlignmentSummary(view, now)
	summaryDigest := digestBytes(string(mustMarshalJSON(t, summary)))

	open := view
	open.Session.Concluded = false
	if _, err := BuildExportContract(open, summaryDigest, now, now, now.Add(time.Hour)); err == nil {
		t.Fatal("unconcluded session exported")
	}
	if _, err := BuildExportContract(view, "not-a-digest", now, now, now.Add(time.Hour)); err == nil {
		t.Fatal("bad summary digest accepted")
	}
	if _, err := BuildExportContract(view, summaryDigest, now, now, now); err == nil {
		t.Fatal("expiry not after now accepted")
	}
}

func mustMarshalJSON(t *testing.T, value any) []byte {
	t.Helper()
	encoded, err := marshalContract(value)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}
