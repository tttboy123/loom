# Phase 2C Final Controller Result

**Date**: 2026-08-08  
**Controller verdict**: `PASS CANDIDATE`  
**Counts**: `P0=0`, `P1=0`, `P2=0`  
**Acceptance state**: independent final review, A4 evidence lock, and Product
Owner sign-off remain required

## Candidate Identity

- Repository: `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild`
- Branch: `codex/loom-platform-slice2`
- Baseline HEAD: `651f156afda37a8e703cbc0396f9f38b7912600b`
- Repair 20 source paths: 51 sorted, unique, existing paths
- Ordered source digest:
  `6fa2f269fefdaed41e60da0c64faf8f173924c427f23c1c5b0937693fab6cfa7`
- Source-lock SHA-256:
  `7ad412e6b0122f3f37dfbbdaa3b5127f22d8b850963a1f1949fd14bf38ec2f1b`
- Staged paths: 0

The source digest and source-lock SHA recomputed exactly after the complete
matrix, signed Release, replacement journey, and journey review. A2-A4 have
added evidence only under the three predeclared append-only paths.

## Product Result

The Repair 20 lock-bound Candidate satisfies the combined Phase 2C Exit
Contract and Repair Amendment at the Controller boundary:

| Gate | Result | Evidence |
|---|---|---|
| Go full serial | PASS | `repair-deterministic-verification.md`, Repair 20 |
| Go full race | PASS | `repair-deterministic-verification.md`, Repair 20 |
| Go vet | PASS | `repair-deterministic-verification.md`, Repair 20 |
| Go module tidy diff | PASS | `repair-deterministic-verification.md`, Repair 20 |
| Locked Go formatting | PASS | `repair-deterministic-verification.md`, Repair 20 |
| Diff hygiene | PASS | `repair-deterministic-verification.md`, Repair 20 |
| Swift full | PASS | 124 XCTest, one intentional visual-audit skip, four Swift Testing tests, zero failures |
| Swift Thread Sanitizer | PASS | same test counts, zero failures, no TSAN finding |
| Signed Release | PASS | `/private/tmp/loom-repair20-release.VRnl2r/Loom.app` |
| Cross-client J1-J8 | PASS | Attempt 022 carried under the reviewed Repair 19 delta boundary |
| Native J9 | PASS | Attempt 023 real keyboard and direct signed-Release AX traversal |
| Native J10 | PASS | Attempt 023 light/dark 900, 1080, and 1440 screenshot matrix |
| Combined J1-J10 | PASS | `PHASE-2C-REPAIR-20-JOURNEY-UI-CARRY-REVIEW-1.md` |

The signed arm64 Release has bundle identifier
`com.earendilworks.loom.local`, executable SHA-256
`a9b02d9e60afa96f8c79e18a73e8e8ca15bde61b1564ae0e1d5086dc7203bc4f`,
ordered bundle digest
`cbef8c096504bb4c6264833c8c0afc0ed5414e94952a8076cf6a5dbeb78a6844f`,
and passes strict deep code-sign verification.

The existing Swift 6 future-compatibility warning for
`QueueCommand<Input>` Sendable conformance is unchanged. It is neither a
Swift 5 build failure nor a Thread Sanitizer finding and is nonblocking for
this locked Candidate.

## P2C-W1 Result - Workspace Shell And Entry

**Controller Product Result**: `PASS CANDIDATE`

- J1, J2, J3, J8, J9, and J10 pass across the required native/TUI boundary.
- Launch is composer-first; Board/governance remains secondary.
- No-folder chat, folder entry, reconnect with preserved view, Recent actions,
  compact rail, keyboard focus, and unique accessibility naming are proven.
- Native and TUI derive truthful service state from daemon responses. The
  final `observer_version_timeout` is a partial observer condition while the
  daemon, Journal, projection, and authoritative Runtime fact remain online;
  it is not presented as an offline fact.
- The light/dark width matrix shows no overlap, clipping, unreadable contrast,
  or displacement of the central conversation.

## P2C-W2 Result - Chat-First Conversation

**Controller Product Result**: `PASS CANDIDATE`

- J4 and J5 pass on the shared daemon fixture through native and real PTY TUI
  clients.
- Ordinary chat creates no Team, Mission, or Run authority fact.
- Agent use remains an explicit trigger. The daemon-authored Team builder,
  preview, compatibility feedback, confirm, and cancel flows are visible and
  operable.
- Team Draft confirmation does not implicitly create a Mission; New Mission is
  a separate explicit governed action.
- Client requests use the strict daemon wire contract, preserve conversation
  continuity, expose typed failures, and do not become a second authority.

## P2C-W3 Result - Governance Side Panel

**Controller Product Result**: `PASS CANDIDATE`

- J5, J6, J7, J8, and J9 pass on the shared daemon fixture.
- Board, Team topology, timeline, decisions, evidence, runtime health, and
  attention are reachable as secondary governance surfaces; no accepted
  visible action is inert or placeholder-only.
- Governance opens, switches, collapses, and closes without cancelling or
  mutating running work. Escape closes the panel and the full Mission Board
  window while preserving the prior workspace view.
- Decision and evidence operations remain daemon-authorized. An executor may
  submit ready-for-review but cannot mark its own WorkItem done.
- Runtime-offline and recovery presentation is truthful, while restart rebuilds
  the preserved authoritative view from the Journal and projection.

## Repair Matrix Closure

Every P0/P1 row in the Repair Amendment has an owned causal test or explicit
cross-client proof and a final green result. Repairs 1-20 remain in the
append-only history, including failed attempts and superseded locks. The final
Candidate closes the observed inert actions, wire-shape failure, dropped
physical spaces, stale service presentation, restart/recovery conflicts,
duplicate Recent accessibility labels, compact layout, keyboard traversal, and
light/dark presentation findings without widening execution authority.

## Operational And Authority Result

**Controller Operational/Trace Result**: `PASS CANDIDATE`

- Attempt 023 records 24 IPC calls, zero failures, and only read-only methods:
  `snapshot`, `chat_thread`, `setup_snapshot`, `permissions_snapshot`, and
  `permissions_attention`.
- Its daemon log records 24 received and 24 pass entries with no authority
  Event ID.
- SQLite integrity is `ok`; Event/Event-ID/idempotency counts are
  `168/168/168`.
- Database and chat state remain byte-identical to Attempt 022.
- Test app, daemon, socket, and socket lock are absent after the journey;
  temporary system appearance and keyboard-navigation settings are restored.
- No Journal, Projection, policy, Grant, Evidence, Scheduler, Provider,
  credential, filesystem, or autonomous-execution authority was added.
- The resident daemon was not modified, restarted, or used for test authority.

## Controller Gate Decision

| Boundary | Controller decision |
|---|---|
| P2C-W1 implementation Result | PASS CANDIDATE |
| P2C-W2 implementation Result | PASS CANDIDATE |
| P2C-W3 implementation Result | PASS CANDIDATE |
| P2C-W1 whole-WorkItem | PASS CANDIDATE |
| P2C-W2 whole-WorkItem | PASS CANDIDATE |
| P2C-W3 whole-WorkItem | PASS CANDIDATE |
| Whole Phase 2C | PASS CANDIDATE |
| ADR-0015 acceptance | PENDING |
| Product Owner sign-off | PENDING |

This Controller result supplies the Product/Controller half of the dual-Result
gate. It does not self-review or self-accept a WorkItem, ADR-0015, or Phase 2C.
An independent Reviewer must return explicit implementation, dual-Result,
whole-WorkItem, and whole-Phase verdicts before the A4 final evidence lock and
Product Owner sign-off request.

