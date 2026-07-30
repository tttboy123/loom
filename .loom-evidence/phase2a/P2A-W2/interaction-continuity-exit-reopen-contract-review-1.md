# P2A-W2 Interaction Continuity Exit Reopen Contract Review 1

**Date**: 2026-07-30
**Reviewer**: fresh independent read-only Reviewer
**Verdict**: FAIL — Contract Repair 1 required

## Reviewed boundary

The Reviewer independently inspected:

- `.loom-evidence/phase2a/P2A-W2/interaction-continuity-exit-reopen-contract.md`;
- the latest Phase 2A section in `docs/CURRENT.md`;
- the parent Phase 2A Exit Contract and original P2A-W2 contract;
- ADR-0011 and ADR-0012;
- attempt-006 result and Result-Evidence Review;
- current native SwiftUI, Bubble Tea, local IPC socket/server and product-daemon
  code.

Repository identity matched
`/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild`,
branch `codex/loom-platform-slice2`, commit
`0546509d2413cff49fdf4a9c795d52b83b84cfa2`.

The Reviewer made no edit, stage, commit, real socket/lock, Keychain, Provider,
network, daemon, resident-service or app action.

## Findings

### P0

None.

### P1 — close implementation was outside the owned boundary

The frozen shutdown order requires exact lock-path removal before releasing the
kernel advisory lock and closing its descriptor. The original ownership list
included `internal/localipc/socket.go`, its tests and server tests, but omitted
`internal/localipc/server.go`.

Current `Server.Close` closes the lock descriptor before removing the lock path.
A concurrent process could therefore acquire the still-linked inode before the
path removal. The Candidate could not satisfy its own stronger ordering without
owning the implementation file.

Required repair:

- add `internal/localipc/server.go` to exact ownership;
- retain remove-before-release ordering;
- add a close-order/concurrent-contender test proving no live or replacement
  lock can be removed after advisory-lock release.

### P2 — abandoned ownership wording was ambiguous

Mandatory RED called the attempt-006-shaped input an `unowned` valid lock while
the algorithm requires a lock owned by the effective user and the GREEN matrix
requires wrong-owner rejection.

Required repair:

- use `abandoned owned` or `unlocked owned` for the stale input;
- keep wrong-owner paths fail-closed and never reclaim them.

## Confirmed non-findings

- The contract is one complete W2 vertical reopen and creates no W2a/W2b/W4.
- The task-first workspace remains pre-execution and creates no new state or
  execution authority.
- The design skills are used only for native product/UX audit; the generated
  newsletter pattern, web font and web palette are rejected.
- Attempt-006 digests and repository identity are exact.
- Provider, Keychain, secret, Journal, StateWriter and W3-lock boundaries remain
  preserved.

## Decision

Contract Review 1 is `FAIL`. Product RED and implementation remain locked until
Contract Repair 1 receives fresh independent Re-review `PASS`.

VERDICT: FAIL
