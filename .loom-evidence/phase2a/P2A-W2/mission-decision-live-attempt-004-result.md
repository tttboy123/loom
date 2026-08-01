# P2A-W2 Mission Decision Live Attempt 004 Result

**Date**: 2026-08-01
**Attempt**: `p2a-w2-mission-decision-live-20260801-004`
**Candidate**: `c94f30b38eb631f3cae6ac36b630e49edcae2c46`
**Verdict**: `PASS — MISSION_DECISION_VIEW_LIFECYCLE_CLOSED`

## Exact start and first discovery

The daemon was invoked exactly once from the preflight-locked binary. It became
the only controlled product daemon, created the private default product socket
and `loomd.sock.lock`, and initialized the fresh SQLite with 73 fixture and
Runtime facts. The first configured Pi observation appended exactly one
`RuntimeInstanceDiscovered` fact for installed Pi `0.82.1`.

The first real product snapshot returned current authoritative view:

```text
e0f70197968587f43f94a73fce0c91f9c2dad84f82287a5639cb1096c138ff87
```

All four prepared decision commands returned by that same snapshot were bound
to that exact view. The snapshot contained five Missions, two online Runtime
instances and no partial/stale marker.

## Real native-window journey

Computer Use launched the exact signed native bundle once. The real window
connected through `LocalProductReadService -> Go IPC Server -> Swift Client`
and displayed:

```text
Orchestrating = 3
Review        = 2

team-auth-allow
team-auth-deny
team-recovery
team-review-missing
team-review-ready
```

Opening `team-auth-deny` proved Mission Room and Team Pulse continuity:

```text
Main / Working / Attempt 1
Decision availability / Open Authorization
```

The real prepared Authorization sheet was bound to
`mission/team-auth-deny`, `main`, Attempt 1 and exposed only:

```text
Not now
Deny
Edit scope
Allow once
```

`Not now` closed the sheet and left total Journal Events exactly `73`.

The Controller reopened the prepared sheet and clicked `Deny` exactly once.
The sheet closed successfully, the Mission Room changed to
`No pending prepared decision`, and total Events became exactly `75`:

```text
ApprovalDecided              +1
WorkItemApprovalResolved     +1
all other Event type counts  unchanged
```

The exact authority facts are:

```text
approval/c28a12bc-2659-49be-ae46-bdd197019c60
  seq 2  ApprovalDecided
  evt-2d24d7013ad40d4c4c606ad30db6cea6

work-item/approval-work-team-auth-deny
  seq 4  WorkItemApprovalResolved
  evt-680fb046a3769fbd32f80bedd71a574b
```

No hidden retry occurred. Replaying the consumed pre-decision command through
the real IPC server returned one fail-closed `conflict`. The next authoritative
snapshot advanced to:

```text
4f138d61feff5a1f38783b67ff8b9ae2e046aa0eb7de02d3b6fcc56cbb524165
```

It contained exactly three remaining prepared decisions, each rebound to that
new view; `team-auth-deny` was absent and only the `team-auth-allow` approval
remained pending.

## Preserved product and safety surfaces

The same native window also proved:

- Runtime & Providers remained reachable;
- Codex showed `Available` through the strict native observer;
- MiniMax remained explicitly `Unconfigured` in this isolated state;
- the UI stated that credentials remain inside the Broker/Keychain boundary;
- `team-review-missing` opened a read-only Review Gate stating that no
  authoritative acceptance command was prepared;
- `Request changes` and `Accept result` remained disabled;
- no raw Grant, credential, secret or hidden reasoning was displayed; and
- the Mission composer remained disabled without accepted messaging authority.

The exact attempt-local TUI connected to the same product socket and displayed
the same five Missions, lanes, Team/Attempt identity and prepared authorization.
It exited normally with status `0` and wrote no Journal fact.

## Normal shutdown and postflight

The native app exited normally through its real GUI. One interrupt was sent to
the only controlled daemon; it exited `0` with:

```json
{
  "completed_cycles": 9,
  "discovery_events": 1,
  "status_events": 0,
  "no_write_cycles": 8
}
```

Postflight proves:

- controlled daemon, native app and TUI processes are absent;
- product socket and `loomd.sock.lock` are absent without manual cleanup;
- no process holds the controlled SQLite;
- SQLite integrity is `ok`;
- total Events are exactly `75` with only the two decision facts added;
- isolation remains empty;
- database is a regular owner-private `0600` file, 81,920 bytes, SHA-256
  `43b8cd7aa6745737cd3d2f37cfce004c616f7e50e96f29b1be208cca1e6ea6e9`;
- manifest remains SHA-256
  `c1025bd00903f4ca5be7de0e541102d00f9b70ea78cf9ea39b679497729bd12e`;
- source-lock copy remains SHA-256
  `04180e4c5c26fb1c7dcc8f8424269b9bbdaaa7a38eaf736be712813c1b5e2b62`;
- daemon, TUI and native executable hashes remain preflight-identical;
- repository `apps/macos/.build` remains absent;
- product source remains clean at Candidate `c94f30b`; and
- the unrelated resident Runtime observer remains running with its own state,
  isolation and no product socket.

## Result

The live defect from Attempt 003 is closed. A freshly served prepared command
remains aligned with the daemon's own discovery-advanced GlobalReadView,
presentation-only defer remains zero-authority, one real native decision
commits once, and replay remains fail-closed.

```text
P2A-W2 replacement canary = PASS
P2A-W2 acceptance         = PENDING INDEPENDENT RESULT-EVIDENCE REVIEW
P2A-W3                    = LOCKED UNTIL THAT REVIEW
P2A-W4                    = DOES NOT EXIST
```
