# P2A-W2 Mission Orchestration Workbench Contract Review 1

**Date**: 2026-07-30
**Reviewer**: fresh independent read-only Contract Reviewer
**Reviewed baseline**:
`bcd27c1b37527e3caed3c2b1e5ec6194f2590512`
**Verdict**: `FAIL`

## Findings

- P0: none.
- P1: one.
- P2: none.

## P1 — decision IPC method is outside the frozen owned boundary

Sections 3 and 4 require a strict local IPC command and product command facade
for Authorization, Review Gate and Recovery. The original owned list included
the daemon handler and Swift client but omitted:

```text
internal/localipc/protocol.go
internal/localipc/protocol_test.go
```

The current closed `validMethod` allowlist rejects every method except the
existing snapshot, timeline, setup, Builder and credential methods before the
daemon handler can dispatch it. A new reviewed decision method therefore cannot
be implemented or causally tested inside the original boundary.

## Required contract-only repair

1. Add `internal/localipc/protocol.go` and its test to exact ownership.
2. Add a mandatory RED proving the new frozen decision method is initially
   rejected by the Go protocol allowlist.
3. Freeze the post-implementation behavior to accept only the exact reviewed
   method name and continue rejecting unknown `decision_*` methods.
4. Keep `internal/localipc/server.go` excluded; no server change is required.

## Confirmed non-findings

The Reviewer found no other blocking issue:

- Mission remains a Projection facade;
- Event, Grant, Evidence, Work and Rules authority remain unowned;
- no W4 or wrapper-only split is created;
- attempt-007 Provider reachability remains a mandatory regression;
- deterministic and single-canary/no-retry gates remain testable.

No product code, test, daemon, client, Provider, Keychain, staging, commit or
live action was performed by the Reviewer.
