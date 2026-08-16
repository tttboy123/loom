# Phase 4 · 4.1 RoundTable — 04 macOS + TUI Dual-Seat Journeys

Status: `PASS`
Date: 2026-08-17

## macOS

- Sidebar **Roundtable** opens `RoundtableWorkbench`: create session → add
  writer + target seats → open round → propose (writer) → relay (moderator) →
  acknowledge (target) → insert (moderator) → conclude (moderator), single-step
  or "Run full journey"; live session state + body digest; concluded shows the
  AlignmentSummary digest and blocks further writes.
- Installed bundle check:
  `strings /Users/lune/Applications/Loom.app/Contents/MacOS/LoomLocalApp |
  grep -cE "Run full journey|Governed handoff ledger|..."` → 2 (workbench
  strings present).
- The Swift client path (same `LocalIPCClient` the workbench uses) is proven
  end-to-end over a real Unix socket by
  `TestStrictSwiftClientRunsRoundtableJourneyOverRealGoServer` (create →
  seats → round → propose → relay → ack → insert → conclude → snapshot, every
  response strictly decoded).

## TUI

- New **Roundtable** screen (tab-cycled). Keys: `n` next step / create session
  entry (default `rt-tui`), `r` replay-refresh, `e` switch session, `q` quit.
  The screen renders session, next step, seats (moderator/writer/target),
  rounds, message status, body digest, and the AlignmentSummary digest after
  conclude. A loading guard makes repeat `n` presses safe (no double-continue).
- Tests (`internal/tui/roundtable_test.go`): journey walks the full sequence;
  after conclude, further advances issue no writes; restart consistency of the
  derived step; wire method names over a real socket.
- Installed-live TUI frames against the real daemon
  (`go run ./cmd/rt-live-journey --tui`): 10 frames ending in
  `Session concluded · digest <64hex>` with the "repeat n does nothing" hint.
  Full frame log: `live/tui-journey-frames.log`.

## Verdict

Gate 4 `PASS`: macOS workbench + TUI dual-seat journey both walk the full
governed lifecycle against the authoritative Journal.
