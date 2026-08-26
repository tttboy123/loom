# Phase 4 · 4.1 RoundTable — 02 Domain / Service Tests

Status: `PASS`
Date: 2026-08-17

## Domain authority (`internal/roundtable`)

`go test ./internal/roundtable/ -count=1` → ok.

Covered lifecycle (roundtable_test.go):
- Full lifecycle + replay: create → add 2 seats → open round → propose
  (pending, digest check) → non-moderator relay rejected → moderator relay →
  wrong-seat ack rejected → target ack → moderator insert → conclude
  (summary artifact read + hash verification) → post-conclude writes rejected →
  replay reproduces the same view/digest.
- Bounded body: > 8 KiB rejected; non-digest artifact ref rejected; unknown
  round rejected.
- Seat retired rejects writes.
- Propose idempotent on exact fact (same message_id/body → same digest;
  different body → conflict).
- Conclude summary canonical + bounded (artifact digest matches journal).

Limits enforced in code: 8 KiB body, 16 digest-only artifact refs, 16 seats,
256 rounds, 1024 messages/round, 1 schema version. Writes go through
`journal.AppendBatchIfStreamHeads` (CAS single-winner); replay is the only
view authority.

## Service / route tests (`cmd/loomd`)

`go test ./cmd/loomd/ -run 'TestProductRoundtable' -count=1` → ok:
- Full lifecycle over the strict route handler with typed error codes
  (`not_moderator`, `seat_unavailable`, `concluded`, `not_found`,
  `invalid_request`, `state_unavailable`).
- Nil service → `state_unavailable`.
- Authenticated Local IPC journey (real Unix socket + real authority).

## Verdict

Gate 2 `PASS`: lifecycle, bounds, moderator gating, post-conclude rejection,
idempotent re-propose, and restart replay consistency all have passing tests.
