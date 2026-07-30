# P2A-W2 Mission Decision Live Attempt 003 Result

**Date**: 2026-07-30
**Attempt**: `p2a-w2-mission-decision-live-20260730-003`
**Candidate**: `700086db1d28837b09dd3f9f72b3bf847f0c400e`
**Verdict**: `FAIL — PREPARED_DECISION_VIEW_STALE_AFTER_DISCOVERY`

## Exact preflight and consumed start

Implementation Review passed before attempt creation. The fresh `0700` attempt
root, `0600` state/manifest, exact 32-file source lock, daemon, TUI, signed
native bundle, Codex executable, installed Pi 0.82.1 and Node identities all
passed preflight.

The daemon was invoked exactly once. PID `39649` became the only controlled
daemon and created:

```text
product socket inode = 76265531
product lock inode   = 76265530
mode                 = 0600
owner                = 501
```

The controlled fixture and first Pi discovery produced a valid 73-Event
baseline with two Runtime discoveries. SQLite integrity was `ok`.

## Native proof completed before the failure

Computer Use launched the exact signed native bundle once. The real window
connected to the product socket and showed:

```text
Orchestrating = 3
Review        = 2

team-auth-allow
team-auth-deny
team-recovery
team-review-missing
team-review-ready
```

Opening `team-auth-deny` showed Team Pulse
`Main / Working / Attempt 1`. The local composer draft
`Inspect current authorization before execution` and Evidence Inspector
selection survived Board round-trip continuity; Send remained disabled.

Runtime & Providers remained reachable and displayed only closed status:

```text
Codex   Unsupported
MiniMax Unconfigured
```

It also displayed the private Broker/Keychain boundary. No raw secret,
credential reference, Grant or hidden reasoning was shown.

The repaired production composition succeeded: `Open Authorization` opened the
real prepared native sheet through `LocalIPCClient -> Go IPC`. The sheet was
bound to `mission/team-auth-deny`, `main`, Attempt 1 and exposed only:

```text
Not now
Deny
Edit scope
Allow once
```

`Not now` closed the sheet and total Events remained exactly 73.

## Blocking live finding

The Controller reopened the same native prepared Authorization sheet and
clicked `Deny` exactly once. The sheet remained visible, matching the
fail-closed Swift behavior for an IPC conflict/failure. Total Events remained
73 and no decision, denial, recovery, generation or Grant Event appeared.

Permitted read-only daemon API diagnostics then proved the exact identity
mismatch:

```text
current authoritative view_version
18ca4c66c9263f54e18c4a25c3c66d95a501d5dcb0bdda42b0d930ae1af32db5

all four prepared decision view_versions
9d3f8a9f6821a6b88e6e9d053622e2a8d510bcd5491f7dc77daa89d72c6f9579
```

The controlled decision registry was prepared while the fixture Projection
still had its pre-observation view. The daemon's first Pi
`RuntimeInstanceDiscovered` fact then advanced the authoritative GlobalReadView
before native submission, but the prepared commands retained the older view
binding.

The server therefore rejected `Deny` as stale, as required by generation/view
fencing. This is correct fail-closed behavior but a live fixture/decision
lifecycle defect: a freshly served prepared command cannot be submitted after
the daemon's own first discovery write.

No second click, direct IPC mutation, direct SQLite mutation, daemon restart,
alternate binary or hidden retry occurred. `Allow once`, Review, Recovery and
TUI parity were not attempted after the blocking step because the frozen stop
rule requires immediate shutdown.

## Normal shutdown and postflight

The failed sheet was closed using its presentation-only close control. The
native app then exited normally. One exact interrupt was sent to the only
controlled daemon; it exited `0`:

```json
{
  "completed_cycles": 19,
  "discovery_events": 1,
  "status_events": 0,
  "no_write_cycles": 18
}
```

Postflight proves:

- controlled daemon PID `39649` and native PID `39995` are absent;
- product socket and lock are absent without manual cleanup;
- no process holds the controlled database;
- isolation contains zero entries;
- SQLite integrity is `ok`;
- total Events remain exactly 73;
- all 20 baseline Event types and counts remain unchanged;
- decision/denial/recovery/generation/Grant Event count is zero;
- database is regular private `0600`, 81,920 bytes, SHA-256
  `ff732896a7764e45d01a4e97960aa02c5022ad902b734a59f35e8d9987876b7d`;
- fixture manifest remains SHA-256
  `2894e17b7b8487bea4f8d32b7d4410d6dcf6a2741bed8f95789384789ed7eaea`;
- source-lock copy remains SHA-256
  `e44f6670abf6c9446678c8945bccfbb2b727fd2c2bebe44e4b85ef0c7e80fe2d`;
- repository `apps/macos/.build` remains absent; and
- product source under `cmd/`, `internal/` and `apps/macos/` remains
  byte-identical to Candidate `700086d`.

The unrelated resident Runtime observer was not signalled, reconfigured or
stopped.

## Result

The repaired native client composition and prepared-sheet display are proven.
The complete native decision exit is not proven because the daemon serves
prepared commands with a view version invalidated by its own first discovery
write.

The single Attempt 003 allowance is consumed:

```text
P2A-W2 = HUMAN_REQUIRED / NOT ACCEPTED
P2A-W3 = LOCKED
P2A-W4 = DOES NOT EXIST
```

The frozen complete reopen forbids restart, an additional live attempt or a
new single-point Amendment. Fresh independent Result-Evidence Review is the
only remaining action permitted in this lineage.
