# SF-W2 cross-client journey

This runbook is the cross-client Exit Gate journey for the single `SF-W2`
(Ephemeral Worker Pools + Lease/Reconciler + routing). It follows the
accepted alternative-verification method (Product Owner instruction
2026-08-04): production native window launched with `--socket --journey-id`,
real PTY TUI, and mutations/reads through the production Swift client over
the real daemon socket; Computer-Use-driven window automation is skipped.

## Scenario assertions (frozen)

1. Two independent workers claim two separate Candidates in parallel; both
   clients observe 2 active workers in the projection.
2. One worker crashes at the after-CAS seam (`AttemptCrashed`,
   `crash_seam=after_cas`, `crash_effect_cardinality=single_effect`); the
   Reconciler reclaims the Job with exactly one new generation
   (`LeaseReclaimed` + `GenerationAdvanced` + `AttemptClaimed`).
3. A stale result from the old generation is rejected
   (`StaleResultRejected`, zero side effects).
4. A failing test (`test_defect`) enters Repair, never Integration.
5. The read-only Reviewer's product write is denied.
6. Restart/reconnect checkpoint: the daemon restarts mid-journey and both
   clients observe the rebuilt worker/attempt state from the Journal with no
   duplicate effects.

## Driving commands (production Swift client over the real socket)

```sh
PROBE=apps/macos/.build/arm64-apple-macosx/release/LoomLocalAppContractProbe
$PROBE --socket ROOT/loomd.sock --workers-snapshot JOURNEY
$PROBE --socket ROOT/loomd.sock --workers-command JOURNEY claim   INPUT.json
$PROBE --socket ROOT/loomd.sock --workers-command JOURNEY crash   INPUT.json
$PROBE --socket ROOT/loomd.sock --workers-command JOURNEY reclaim INPUT.json
$PROBE --socket ROOT/loomd.sock --workers-command JOURNEY reject_stale INPUT.json
$PROBE --socket ROOT/loomd.sock --workers-command JOURNEY result   INPUT.json
$PROBE --socket ROOT/loomd.sock --workers-command JOURNEY review_write_denied INPUT.json
```

The real PTY TUI navigates to the Workers screen (Tab through the screen
cycle, `r` refresh, `q` quit) and both clients observe the same projection.
The GUI evidence surface follows the frozen
`P3A-W1-GUI-EVIDENCE-SURFACE-AMENDMENT.md`.
