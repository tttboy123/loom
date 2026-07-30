# P2A-W2 Mission Orchestration Live Closure Attempt 002 Result

**Date**: 2026-07-30
**Lineage**: `p2a-w2-mission-workbench-live-20260730-002`
**Candidate commit**: `d0252064e43dc2c7d9e047aaa36942ef7b92d97b`
**Verdict**: `FAIL — NATIVE_DECISION_CLIENT_PROTOCOL_UNAVAILABLE`

## Exact preflight and one consumed start

Every reviewed preflight gate passed before the first daemon invocation. The
lineage used the frozen regular Codex executable, source lock, daemon, TUI,
signed native bundle and controlled fixture identities recorded by:

- `mission-orchestration-live-closure-preflight.md`;
- `mission-orchestration-live-closure-preflight-review.md`;
- `mission-orchestration-live-closure-preflight-repair-1-result.md`; and
- `mission-orchestration-live-closure-preflight-repair-1-result-review.md`.

The daemon was invoked exactly once with the frozen attempt root and manifest.
It created private product socket inode `76128578` and private lock inode
`76128577`. PID `5702` was the only listener. The separate resident Runtime
observer was not signalled, reconfigured or stopped.

The daemon appended exactly one controlled
`RuntimeInstanceDiscovered` Event for the installed Pi 0.82.1 Runtime. It then
completed 25 cycles with one discovery write, zero status writes and 24
no-write cycles.

## Native Mission Workbench proof

Computer Use opened the exact signed native bundle:

```text
/Users/lune/Library/Application Support/Loom/
p2a-w2-mission-workbench-live-20260730-002/native/Loom.app
```

The real window connected to the product socket and showed the Journal-backed
five-lane Mission Board:

```text
Orchestrating = 3
Review        = 2

team-auth-allow
team-auth-deny
team-recovery
team-review-missing
team-review-ready
```

The Controller opened `team-auth-allow`, inspected Team Pulse
`Main / Working / Attempt 1`, entered the local draft
`Inspect current authorization before execution`, selected the Evidence
Inspector and Plan-only proposal mode, returned to the Board, reopened the
Mission and observed the same draft and Evidence Inspector selection. The
composer remained non-authoritative and Send remained disabled.

Provider Manage was reachable through the native rail. It displayed:

```text
Codex   Unsupported
MiniMax Unconfigured
```

and the explicit private Broker/Keychain boundary. No raw credential, OAuth
material, API key or credential reference was displayed or entered.

The missing-Evidence Mission opened its local read-only Review Gate. The sheet
stated that accepted terminal Evidence was required and kept both
`Request changes` and `Accept result` disabled. Closing with `Not now`
preserved the exact 73-Event baseline.

## Blocking live finding

The native `Open Authorization` action for a real prepared command did not
open a decision sheet. The same failure necessarily blocks `Deny`,
`Allow once` and `Start New Attempt`.

Read-only diagnostics isolated the failure without substituting for the
required product interaction:

1. the daemon snapshot exposed four valid prepared commands;
2. a framed read-only `mission_decision` request for
   `team-auth-deny` returned the complete prepared authorization sheet with
   `ok=true`;
3. the native `LocalProductStore` nevertheless retained no active decision
   sheet; and
4. no Event was appended.

The committed client declaration is:

```swift
public final class LocalIPCClient:
    LocalProductClientProtocol,
    LocalProductSetupClientProtocol
```

Although `LocalIPCClient` implements both `readMissionDecision` and
`decideMission`, it does not declare
`LocalProductDecisionClientProtocol`. `LocalProductStore` obtains its decision
client only through:

```swift
decisionClient = client as? LocalProductDecisionClientProtocol
```

The cast therefore returns `nil` in the real native app. This is a product
composition defect in the frozen Candidate, not an IPC, Journal, fixture,
Computer Use or environment failure.

The Controller did not hot-patch the running bundle, invoke a mutation through
direct IPC, restart the daemon, relaunch another app binary or consume a
second lineage.

## GUI/TUI consistency and authoritative stop state

The real TUI connected to the same socket and showed the same five Missions:

```text
team-auth-allow       Orchestrating  Running
team-auth-deny        Orchestrating  Running
team-recovery         Orchestrating  Awaiting recovery
team-review-missing   Review         Ready for review
team-review-ready     Review         Ready for review
```

Its selected Mission also showed `Main / Working / Attempt 1` and the prepared
Authorization. The TUI exited normally with `q`.

The gate-sensitive mission/runtime counts remained:

```text
total Events                         73
RuntimeInstanceDiscovered             2
ApprovalRequested                     2
WorkItemApprovalPaused                2
RunClaimed                            5
RunStarted                            3
RunTerminalCommitted                  3
EvidenceSubmitted                     3
```

The controlled fixture also retained its expected planning, capacity, WorkItem
and identity-index Events; total Event count remained 73.

No decision, recovery, new Attempt, new generation, Grant or execution Event
was created during UI interaction. Authoritative Event payload scans returned
zero secret markers, hidden-reasoning markers and raw-Grant markers.

## Normal shutdown and postflight

The native app exited normally through its application quit command. The TUI
had already exited `0`. One exact interrupt was sent to the only controlled
daemon; it exited `0` and reported:

```json
{
  "completed_cycles": 25,
  "discovery_events": 1,
  "status_events": 0,
  "no_write_cycles": 24
}
```

Postflight proves:

- controlled daemon PID `5702` and native PID `8865` are absent;
- product socket and lock are both absent without manual cleanup;
- no process holds the controlled SQLite file;
- the isolation root is empty;
- SQLite `integrity_check = ok`;
- the database is regular private `0600`, 81,920 bytes, SHA-256
  `41f83be7bde3225a60729f740cfbc2990582ad9e1be0d6732410a8bc076e7fc8`;
- the fixture manifest remains SHA-256
  `194e698a4c6954bc98e38c1380e58798be22eee07c01214c98e107d866e08970`;
- product source under `cmd/`, `internal/` and `apps/` remains byte-identical
  to Candidate `d0252064`; and
- the reviewed quarantined Swift cache remains preserved outside the
  repository.

## Result

The unique replacement lineage proves the real native Mission Board, Mission
Room continuity, Team Pulse, Provider Manage route, fail-closed missing
Evidence gate, real TUI parity and graceful lifecycle cleanup. It does not
prove authoritative Authorization or Recovery because the native client fails
to adopt the already-implemented decision protocol.

The one daemon/native-window allowance is consumed:

```text
P2A-W2 = HUMAN_REQUIRED / NOT ACCEPTED
P2A-W3 = LOCKED
P2A-W4 = DOES NOT EXIST
```

There is no retry, alternate binary or live hot repair under this lineage.
Fresh independent read-only Result-Evidence Review is required for this failed
outcome. Any product repair requires a reviewed complete P2A-W2 reopen; it may
not be disguised as a single-point Amendment.
