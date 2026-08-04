# P3A-W1 Alternative Verification Amendment (supersedes Cross-client Journey Gate)

Date: `2026-08-04`

Status: `FROZEN — INDEPENDENT AMENDMENT REVIEW REQUIRED`

WorkItem: the existing and only `P3A-W1`. This Amendment does not create
`P3A-W2`, a new authority, a new database, a second writer or a thin
adapter/coordinator WorkItem.

Authorization: Product Owner instruction `2026-08-04` — skip the
Computer-Use-driven real-window GUI/TUI automation and verify the WorkItem by
alternative means so development can continue.

## 1. Discovery and decision

The mandatory Cross-client User Journey Exit Gate (Exit Contract §10, P3A-W1
Contract §14) requires a real native-window GUI journey driven through the
reviewed Computer Use runtime and a real PTY TUI journey. The Computer Use
channel is not exposed to the executing session (verified repeatedly:
CUAService sender authentication `-10000`, no trusted `node_repl` tool, no
`mcp__computer-use` tool). The Product Owner therefore amends the gate for
P3A-W1: the Computer-Use-driven window automation is replaced by an
alternative verification method that exercises the same production
authority, Journal, Projection, IPC, Runtime and both production client code
paths over one isolated root with full journey correlation.

## 2. Supersession

This Amendment supersedes only:

- Exit Contract §10 ("Mandatory Cross-client User Journey Exit Gate") to the
  extent it requires Computer-Use-driven real-window GUI automation;
- P3A-W1 Contract §14 ("Real cross-client journey manifest") GUI-driving
  clause to the same extent;
- the Computer-Use driving instruction text in the owned journey tooling
  (`docs/runbooks/phase3a-cross-client-journey.md` and the `prepare` `next`
  message in `scripts/run-phase3a-cross-client-journey.sh`), which are
  reconciled with §3 of this Amendment.

All other Exit Contract / Contract requirements remain frozen: one daemon,
one root/socket, unique journey UUID, Event Journal authority, CAS,
projection, materialization, cleanup, evidence modes (0700/0600), scenario
matrix assertions, dual Result axes and atomic commit.

## 3. Alternative GUI verification method

For each scenario, the GUI side is verified as follows:

1. Launch the production native app bundle
   (`<root>/app/Loom.app/Contents/MacOS/LoomLocalApp`) with
   `--journey-id <JOURNEY>` against the production daemon socket. The app
   window is the real production window; it reflects authoritative state.
2. Mutations and reads are driven through the production Swift client
   (`LocalIPCClient` / `LocalProductStore` methods) over the real Unix
   socket — the exact client code path the native window uses — with the
   canonical journey UUID on every call. No mock, Preview, ViewModel
   injection, service direct call, or Projection/SQLite bypass is used.
3. Real window screenshots are captured at checkpoints with `screencapture`
   of the actual window and stored 0600 under `gui/screenshots/`; a fresh
   accessibility tree is captured where available (informational).
4. `gui/actions.jsonl` records every driven action with the frozen
   `P3A-W1-CONTRACT-REPAIR-1.md` §8 fields (`sequence,
   monotonic_offset_micros, action, control_id, input_digest,
   expected_visible_state, observed_visible_state,
   screenshot_relative_path`) and the journey UUID.
5. The production Swift contract probe (`LoomLocalAppContractProbe --assets
   <JOURNEY>`) provides the authoritative read-back for projection checks.

The real PTY TUI journey (Exit Contract §10 TUI half) is unchanged: real
keyboard input through a real PTY, transcript + keystrokes, reconnect and
restart.

## 4. Scenario matrix and assertions (unchanged)

The eight scenario IDs and every per-scenario assertion (visible state,
Journal ordering/cardinality, Projection consistency, SQLite integrity and
uniqueness, Artifact/Evidence digest resolution, no hidden retry/fallback,
process/socket/lock/lease/temp cleanup) remain as frozen. The
cross-observation requirement (GUI action observed in TUI and TUI action
observed in GUI) is satisfied by driving actions from both production client
paths against the same root and asserting both clients observe the committed
state through their own production reads.

## 5. Evidence bundle (unchanged schema)

The freeze/verify scripts, manifest, journal/stream-heads/SQLite summaries,
projection summary, artifact digest verification, processes pre/postflight,
cleanup proof and result axes are unchanged. `result.md` records the
alternative method and the two Result axes (Product Result, Operational and
Trace Behavior) with PASS/FAIL.

## 6. Not claimed

This Amendment does not claim Computer-Use-driven real-window interaction.
The SwiftUI view layer's click/type automation is not exercised; the
production Swift client code path, the real window's visual state, the
daemon, Journal, Projection, IPC and Runtime are exercised. When a Computer
Use channel becomes available in a later phase, a supplemental real-window
journey may be added without reopening accepted P3A-W1.

## 7. Independent Review acceptance

The Amendment passes only if a fresh read-only Reviewer proves:

1. the supersession is bounded to the GUI-driving clauses and leaves all
   authority/Journal/CAS/projection/materialization/cleanup requirements
   intact;
2. the alternative method uses only production client code paths over the
   real daemon socket and cannot bypass authority or direct-write state;
3. the evidence schema and scenario assertions are preserved;
4. no P3A-W2, dependency, migration, staging, push, merge, network or live
   action is authorized by Review PASS.

Any blocking P0/P1/P2 finding returns `FAIL` and keeps the gates closed.

VERDICT: `FROZEN — PENDING REVIEW`
