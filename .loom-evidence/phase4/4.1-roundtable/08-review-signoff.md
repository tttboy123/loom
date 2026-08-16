# Phase 4 · 4.1 RoundTable — 08 Independent Review & Sign-off

Status: `REVIEW COMPLETE / SIGN-OFF PENDING OPERATOR`
Date: 2026-08-17

## Independent review (separate reviewer pass)

An independent read-only reviewer (separate from the implementing writer)
verified the diff, the live journal/evidence, and re-ran the relevant suites.
Its initial verdict was **BLOCK**; both blockers and the actionable nits were
addressed and re-verified:

- **B1 — AddSeats step not restart-safe (partial commit wedges the session):**
  fixed in `internal/tui/roundtable.go` — the step is now idempotent per seat
  (skips seats already present in the replayed view), so a writer-committed /
  target-failed partial step retries cleanly after restart. Covered by
  `TestTUIAddSeatsIsIdempotentPerSeat`.
- **B2 — stale cached view on session switch / errors:** fixed in
  `internal/tui/model.go` — entering a different session id clears
  `roundtableView` + error, and command errors reload the authoritative
  Journal view so step derivation self-heals. Covered by
  `TestTUISessionSwitchClearsStaleView`.
- **N1 — evidence publish before CAS append:** deliberate publish-then-append
  ordering documented in `internal/roundtable/authority.go` (a CAS conflict
  can only leave an unreferenced digest-bound orphan, never a fact pointing at
  a missing artifact).
- **N2 — ConcludedAt not carried in replay digest:** recorded in the Journal
  fact + artifact; replay verification uses the recorded digest. Accepted
  asymmetry for 4.1, noted for 4.2 summary re-derivation.
- **N3 — prior findings confirmed accurate** (artifact_refs normalization,
  concluded_at, loading guard, exported wire types) via live journal/artifact
  checks.
- **N4 — evidence accuracy:** permissions wording corrected (live logs 0600;
  summary .md follow repo convention).
- **N5 — UX:** `--tui` driver uses a unique session id; TUI default `rt-tui`
  recovers via `e` + a new id. Non-blocking.

Post-fix re-verification: `go test ./internal/tui/` (all roundtable tests
including the two blocker regressions) + `go test ./internal/roundtable/`
+ race + full Go suite (serial) all green; installed live journey + TUI
journey re-run and captured (live/).

Review verdict after fixes: **APPROVE** (no remaining blockers).

## Sign-off

- Product Owner / operator sign-off: **not yet given** — this evidence file is
  the sign-off request. The 4.1 acceptance gates are all green in evidence;
  4.1 is NOT marked complete until the operator confirms.
- Next: upon sign-off, proceed to 4.2 (import/export contract for
  ContextPacket / AlignmentSummary: expiry, provenance, dual-surface
  permissions, idempotency).
