# Phase 4 · 4.1 RoundTable — 05 Installed Live E2E

Status: `PASS`
Date: 2026-08-17
App: `/Users/lune/Applications/Loom.app` (v0.5.3, rebuilt + reinstalled for
this slice; previous preserved at `Loom.app.previous`)

## Method

`go run ./cmd/rt-live-journey --journey rt-live-e2e-003` over the installed
daemon's real Unix socket (`~/Library/Application Support/Loom/run/loomd.sock`).

## Journey log (live/installed-ipc-journey-rt-live-e2e-003.log)

```
created session rt-live-e2e-003
step add seats ok
step open round ok
step propose ok
step relay ok
step ack ok
step insert ok
step conclude ok
session=rt-live-e2e-003 moderator=seat-moderator concluded=true
  digest=dc75763dd0d209eeac7d99dbb0ef00776079de4e9bb3c85ce8f57294f06a391f
  seat seat-moderator available=true
  seat seat-target available=true
  seat seat-writer available=true
  msg msg-1 seat-writer->seat-target status=inserted
restart-consistent digest dc75763... (concluded=true)
post-conclude writes rejected
```

## AlignmentSummary artifact verification

- Journal concluded fact (read-only sqlite on
  `~/Library/Application Support/Loom/state/loom.db`):
  `summary_digest=3e5911c1b903b3b6ccb9ce0abbe09f060ec36e21d59eefc0631cf0ac35fdbbba`
  (== `artifact_digest`).
- Evidence artifact exists at
  `state/evidence/artifacts/sha256/3e/3e5911c1...` and
  `shasum -a 256` matches the digest exactly.
- Artifact is canonical: `concluded_at` populated, `artifact_refs: []`
  (never null), bounded message body ≤ 8 KiB, no credential/prompt/Provider
  body (see 06).
- Journal roundtable stream (rt-live-e2e-002/003): 9 facts in order
  SessionCreated → SeatAdded×2 → RoundOpened → MessageProposed →
  MessageRelayed → MessageAcknowledged → MessageInserted → Concluded.

## Restart consistency

- `RoundtableReadView` re-read after conclude returns the same digest
  (`restart-consistent digest ...`), same concluded state.
- Post-conclude propose and re-conclude both rejected (no double-continue).
- TUI `r` replays the same view; repeat `n` after conclude issues no writes
  (covered by `TestTUIAdvanceStepDoesNotDoubleContinueAfterConclude` and the
  live frames log).

## Verdict

Gate 5 `PASS`: create→seats→round→propose→relay→ack→insert→conclude over the
real installed socket; AlignmentSummary exists and its digest matches the
Journal; restart yields the same view/decision; repeats do not double-continue.
