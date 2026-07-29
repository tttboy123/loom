# P2A-W1 Native Product Experience Contract Review

**Date**: 2026-07-29
**Review mode**: fresh, independent, read-only
**Verdict**: `FAIL`
**Live authority**: none

## Findings

### P1 - bounded source-reopen authority was implicit

The terminal P2A-W1 record explicitly authorized no source change or new point
closure. The proposed contract called itself a vertical reopen but did not
record the Product Owner's new bounded UI authorization or say precisely which
old prohibition it superseded.

Repair requirement: record that the user's 2026-07-29 request to optimize the
current project UI is the sole bounded authority to reopen the exact owned
Swift presentation files. Preserve the failed canary, allowance `0`, W2 lock,
and every non-UI prohibition.

### P1 - actual SwiftUI acceptance was outside the test surface

The contract froze keyboard, target-size, Dynamic Type, Reduce Motion,
light/dark, and multi-state claims, but the existing Swift test target depended
only on `LoomLocalAppCore`. `ContentView` remained executable-only. Core
presentation tests plus a screenshot could not prove the actual view boundary.
The proposed fallback accessibility-tree audit also required launching the app,
whose `.task` immediately attempts a refresh.

Repair requirement: own a non-connecting SwiftUI test/render boundary or narrow
the claims to exactly what can be proved. No audit may launch a view that can
reach the product socket.

### P2 - "recent work" was not derivable

`LocalProductRunSummary` has no time or recency field. Calling source-ordered
Runs "recent" conflicted with the pure projection and no-invention rules.

Repair requirement: name the group as source-ordered work activity unless an
accepted authority already guarantees recency.

### P2 - Compare semantics were incomplete

The proposed contract defined the fewer-than-two Runs case but not the
two-or-more case. The current schema has no authoritative comparison result.

Repair requirement: either freeze a complete deterministic comparison from
named existing fields or keep Compare unavailable and absent for every state.

## Confirmed boundaries

The proposal otherwise remains one P2A-W1 vertical reopen, not P2A-W4. It
preserves the failed canary, allowance `0`, and W2 lock; grants no live,
Provider, Runtime, Journal, or schema-normalization authority; prohibits fake
actions; and uses Multica only as product-architecture inspiration.

`VERDICT: FAIL`
