package roundtable

import (
	"encoding/json"
	"errors"
	"time"
)

// Roundtable export/import contract (4.2). An ExportContract is a bounded,
// versioned ContextPacket that carries the concluded session's provenance,
// an expiry window, the digest chain (AlignmentSummary digest + view digest),
// and the canonical AlignmentSummary. It is content-addressed: Digest is the
// SHA-256 of the canonical JSON without the Digest field, and import verifies
// provenance + digest + expiry before accepting the packet.
const ExportSchemaVersion = 1

var (
	ErrInvalidExportContract = errors.New("invalid Roundtable export contract")
	ErrExportContractExpired = errors.New("Roundtable export contract expired")
	ErrExportContractNotYet  = errors.New("Roundtable export contract not yet valid")
	ErrExportContractDigest  = errors.New("Roundtable export contract digest mismatch")
)

// ExportContract is the governed-handoff export document. It carries no
// credential, raw Grant, or hidden reasoning; message bodies stay bounded and
// artifact references are digest-only.
type ExportContract struct {
	SchemaVersion int              `json:"schema_version"`
	SessionID     string           `json:"session_id"`
	ModeratorSeat string           `json:"moderator_seat"`
	ConcludedAt   time.Time        `json:"concluded_at"`
	NotBefore     time.Time        `json:"not_before"`
	ExpiresAt     time.Time        `json:"expires_at"`
	SummaryDigest string           `json:"summary_digest"`
	ViewDigest    string           `json:"view_digest"`
	Summary       AlignmentSummary `json:"summary"`
	Digest        string           `json:"digest"`
}

// marshalContract is the canonical JSON marshaller for contract documents.
func marshalContract(value any) ([]byte, error) {
	return json.Marshal(value)
}

// contractWithoutDigest returns a copy with an empty Digest so the content
// digest covers every field except the digest itself (no self-reference).
func contractWithoutDigest(contract ExportContract) ExportContract {
	contract.Digest = ""
	return contract
}

// BuildExportContract materializes a concluded session view into an export
// contract. summaryDigest must be the Journal-recorded AlignmentSummary digest
// and concludedAt the Journal-recorded conclusion time; the view must rebuild
// the exact same summary (provenance). now is NotBefore and expiresAt must be
// after now.
func BuildExportContract(
	view View,
	summaryDigest string,
	concludedAt time.Time,
	now time.Time,
	expiresAt time.Time,
) (ExportContract, error) {
	if !view.Session.Concluded ||
		!validRoundtableID(view.Session.ID, MaxSessionIDBytes) ||
		!validRoundtableID(view.Session.ModeratorSeat, MaxSeatIDBytes) ||
		!validSHA256Digest(summaryDigest) ||
		!validSHA256Digest(view.Digest) ||
		concludedAt.IsZero() || concludedAt.Location() != time.UTC ||
		now.IsZero() || now.Location() != time.UTC ||
		!expiresAt.After(now) {
		return ExportContract{}, ErrInvalidExportContract
	}
	summary := BuildAlignmentSummary(view, concludedAt)
	summaryJSON, err := marshalContract(summary)
	if err != nil {
		return ExportContract{}, ErrInvalidExportContract
	}
	if digestBytes(string(summaryJSON)) != summaryDigest {
		return ExportContract{}, ErrExportContractDigest
	}
	contract := ExportContract{
		SchemaVersion: ExportSchemaVersion,
		SessionID:     view.Session.ID,
		ModeratorSeat: view.Session.ModeratorSeat,
		ConcludedAt:   summary.ConcludedAt,
		NotBefore:     now,
		ExpiresAt:     expiresAt,
		SummaryDigest: summaryDigest,
		ViewDigest:    view.Digest,
		Summary:       summary,
	}
	canonical, err := marshalContract(contractWithoutDigest(contract))
	if err != nil {
		return ExportContract{}, ErrInvalidExportContract
	}
	contract.Digest = digestBytes(string(canonical))
	return contract, nil
}

// ValidateImportContract verifies an export contract at time now: schema and
// identity bounds, the expiry window (NotBefore <= now <= ExpiresAt), the
// provenance chain (embedded summary hashes to SummaryDigest, ViewDigest is a
// valid digest), and the canonical content digest. It rejects expired,
// not-yet-valid, tampered, and foreign/malformed packets.
func ValidateImportContract(contract ExportContract, now time.Time) error {
	if contract.SchemaVersion != ExportSchemaVersion ||
		!validRoundtableID(contract.SessionID, MaxSessionIDBytes) ||
		!validRoundtableID(contract.ModeratorSeat, MaxSeatIDBytes) ||
		contract.ConcludedAt.IsZero() ||
		!validSHA256Digest(contract.SummaryDigest) ||
		!validSHA256Digest(contract.ViewDigest) ||
		contract.NotBefore.IsZero() || contract.ExpiresAt.IsZero() {
		return ErrInvalidExportContract
	}
	if now.Before(contract.NotBefore) {
		return ErrExportContractNotYet
	}
	if now.After(contract.ExpiresAt) {
		return ErrExportContractExpired
	}
	summaryJSON, err := marshalContract(contract.Summary)
	if err != nil {
		return ErrInvalidExportContract
	}
	if digestBytes(string(summaryJSON)) != contract.SummaryDigest {
		return ErrExportContractDigest
	}
	canonical, err := marshalContract(contractWithoutDigest(contract))
	if err != nil {
		return ErrInvalidExportContract
	}
	if digestBytes(string(canonical)) != contract.Digest {
		return ErrExportContractDigest
	}
	return nil
}
