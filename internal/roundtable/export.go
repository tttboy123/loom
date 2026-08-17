package roundtable

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"loom-pi-rebuild/internal/journal"
)

// ExportResult is the outcome of exporting a concluded session: the export
// contract was published as a content-addressed Evidence artifact and the
// export fact appended to the Journal (CAS).
type ExportResult struct {
	SchemaVersion int       `json:"schema_version"`
	SessionID     string    `json:"session_id"`
	ExportID      string    `json:"export_id"`
	Digest        string    `json:"digest"`
	NotBefore     time.Time `json:"not_before"`
	ExpiresAt     time.Time `json:"expires_at"`
}

// ImportResult is the outcome of importing an export contract: the packet was
// verified (schema, provenance digest chain, expiry), a read-only view was
// reconstructed, and (when the session exists in this Journal) an idempotent
// import fact was appended.
type ImportResult struct {
	SchemaVersion  int    `json:"schema_version"`
	SessionID      string `json:"session_id"`
	ImportID       string `json:"import_id"`
	ContractDigest string `json:"contract_digest"`
	ViewDigest     string `json:"view_digest"`
	Recorded       bool   `json:"recorded"`
	View           View   `json:"view"`
}

// ExportSession publishes a bounded export contract for a concluded session.
// The contract is content-addressed (Digest = SHA-256 of the canonical JSON);
// publishing is deliberately before the Journal CAS append so a committed
// export fact never references a missing artifact (a concurrent CAS failure
// can only leave an unreferenced digest-bound orphan).
func (authority *Authority) ExportSession(
	ctx context.Context,
	sessionID string,
	expiresAt time.Time,
) (ExportResult, error) {
	var result ExportResult
	_, err := authority.apply(ctx, sessionID, func(state sessionState) (View, fact, error) {
		if !state.found {
			return View{}, fact{}, ErrRoundtableSessionNotFound
		}
		if !state.session.Concluded ||
			state.concludedSummaryDigest == "" || state.concludedAt.IsZero() {
			return View{}, fact{}, ErrRoundtableConflict
		}
		now := authority.now()
		if expiresAt.IsZero() || expiresAt.Location() != time.UTC ||
			!expiresAt.After(now) {
			return View{}, fact{}, ErrInvalidExportContract
		}
		contract, err := BuildExportContract(
			state.view, state.concludedSummaryDigest, state.concludedAt, now, expiresAt,
		)
		if err != nil {
			return View{}, fact{}, err
		}
		// The artifact is the canonical digest-less document so its SHA-256
		// equals the content address (contract.Digest) recorded in the fact.
		encoded, err := marshalContract(contractWithoutDigest(contract))
		if err != nil {
			return View{}, fact{}, ErrInvalidExportContract
		}
		if _, err := authority.evidence.Publish(
			ctx, bytes.NewReader(encoded), contract.Digest,
		); err != nil {
			return View{}, fact{}, err
		}
		exportID := authority.deterministicID(
			"roundtable-export", sessionID, contract.Digest, now.Format(time.RFC3339Nano),
		)
		payload, err := json.Marshal(sessionExportedPayload{
			SchemaVersion: SchemaVersion, SessionID: sessionID,
			ExportID: exportID, Digest: contract.Digest,
			NotBefore: contract.NotBefore, ExpiresAt: contract.ExpiresAt,
			ExportedAt: now,
		})
		if err != nil {
			return View{}, fact{}, err
		}
		result = ExportResult{
			SchemaVersion: SchemaVersion, SessionID: sessionID,
			ExportID: exportID, Digest: contract.Digest,
			NotBefore: contract.NotBefore, ExpiresAt: contract.ExpiresAt,
		}
		return cloneView(state.view), fact{
			Event: journalEvent(
				authority.deterministicID("roundtable-session-exported", sessionID, exportID),
				sessionStream(sessionID), state.head.Sequence+1,
				"roundtable.export."+sessionID+"."+exportID,
				FactSessionExported, now, "", state.head.EventID, payload,
			),
			commandID: "export:" + sessionID + ":" + exportID,
		}, nil
	})
	if err != nil {
		return ExportResult{}, err
	}
	return result, nil
}

// ImportSession verifies an export contract and reconstructs a read-only view.
// It never creates a second writable session: when the session already exists
// in this Journal an import fact is appended (idempotent per contract digest —
// re-importing the same packet returns the same result and appends nothing); a
// foreign/unknown session is accepted as a verified read-only copy with no
// fact appended.
func (authority *Authority) ImportSession(
	ctx context.Context,
	contract ExportContract,
	correlationID string,
) (ImportResult, error) {
	now := authority.now()
	if err := ValidateImportContract(contract, now); err != nil {
		return ImportResult{}, err
	}
	if !validCorrelationID(correlationID) {
		return ImportResult{}, ErrInvalidRoundtableMessage
	}
	view := importView(contract)
	importID := authority.deterministicID(
		"roundtable-import", contract.SessionID, contract.Digest,
	)
	recorded := false
	_, err := authority.apply(ctx, contract.SessionID, func(state sessionState) (View, fact, error) {
		if !state.found {
			// Unknown session: verified read-only reconstruction, no fact (no
			// second session is created in this Journal).
			return state.view, fact{}, nil
		}
		if _, already := state.importedContracts[contract.Digest]; already {
			// Idempotent: the same packet was already imported.
			return state.view, fact{}, nil
		}
		payload, err := json.Marshal(sessionImportedPayload{
			SchemaVersion: SchemaVersion, SessionID: contract.SessionID,
			ImportID: importID, ContractDigest: contract.Digest,
			ViewDigest: contract.ViewDigest, ImportedAt: now,
		})
		if err != nil {
			return View{}, fact{}, err
		}
		recorded = true
		return state.view, fact{
			Event: journalEvent(
				authority.deterministicID("roundtable-session-imported", contract.SessionID, importID),
				sessionStream(contract.SessionID), state.head.Sequence+1,
				"roundtable.import."+contract.SessionID+"."+importID,
				FactSessionImported, now, correlationID, state.head.EventID, payload,
			),
			commandID: "import:" + contract.SessionID + ":" + importID,
		}, nil
	})
	if err != nil {
		return ImportResult{}, err
	}
	return ImportResult{
		SchemaVersion: SchemaVersion, SessionID: contract.SessionID,
		ImportID: importID, ContractDigest: contract.Digest,
		ViewDigest: contract.ViewDigest, Recorded: recorded, View: view,
	}, nil
}

// importView reconstructs a read-only session view from a verified export
// contract. Timestamps beyond concluded_at are not carried by the contract;
// the body digest is re-derived deterministically from session/message/body.
func importView(contract ExportContract) View {
	seats := make(map[string]Seat, len(contract.Summary.Seats))
	for _, seat := range contract.Summary.Seats {
		seats[seat.ID] = Seat{ID: seat.ID, DisplayName: seat.DisplayName, Available: true}
	}
	messages := make(map[string]Message)
	rounds := make([]Round, 0, len(contract.Summary.Rounds))
	for _, round := range contract.Summary.Rounds {
		roundMessages := make([]Message, 0, len(round.Messages))
		for _, message := range round.Messages {
			rebuilt := Message{
				ID: message.ID, RoundID: message.RoundID,
				WriterSeat: message.WriterSeat, TargetSeat: message.TargetSeat,
				Body:         message.Body,
				ArtifactRefs: normalizedArtifactRefs(message.ArtifactRefs),
				BodyDigest: digestBytes(
					contract.SessionID, message.ID, message.Body,
				),
				Status: message.Status,
			}
			roundMessages = append(roundMessages, rebuilt)
			messages[message.ID] = rebuilt
		}
		rounds = append(rounds, Round{
			ID: round.ID, Sequence: round.Sequence,
			MessageCount: len(roundMessages), Messages: roundMessages,
		})
	}
	return View{
		Session: Session{
			ID: contract.SessionID, ModeratorSeat: contract.ModeratorSeat,
			Title: contract.Summary.Title, CreatedAt: contract.ConcludedAt,
			Concluded: true,
		},
		Seats: seats, Rounds: rounds, Messages: messages,
		Digest: contract.ViewDigest,
	}
}

func journalEvent(
	id string,
	streamID string,
	sequence int64,
	idempotencyKey string,
	eventType string,
	emittedAt time.Time,
	correlationID string,
	causationID string,
	payload []byte,
) journal.Event {
	return journal.Event{
		ID: id, StreamID: streamID, Seq: sequence,
		IdempotencyKey: idempotencyKey, Type: eventType,
		SchemaVersion: SchemaVersion, EmittedAt: emittedAt,
		CorrelationID: correlationID, CausationID: causationID,
		PayloadJSON: payload,
	}
}
