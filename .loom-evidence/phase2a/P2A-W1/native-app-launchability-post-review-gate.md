# P2A-W1 Native App Launchability Post-Review Gate

**Date**: `2026-07-28`
**Status**: `PASS — DETERMINISTIC REPAIR CLOSED — LIVE LOCKED`
**Native live allowance**: `0`
**Candidate installed**: no
**P2A-W2**: locked

## Closed lineage

1. consumed native-window canary:
   `FAIL — ROLLED_BACK — HUMAN_REQUIRED`;
2. independent Result-Evidence Re-review 2: `PASS`;
3. immutable crash diagnosis:
   `DYLD — missing LC_UUID load command`;
4. Launchability Closure Contract Review 1: `FAIL`;
5. Contract Review Repair 1: pre-spawn RED and crash-report accounting;
6. fresh Contract Re-review 2: `PASS`;
7. mandatory pre-spawn RED: `PASS`;
8. deterministic GREEN: `PASS`;
9. fresh independent Implementation Review: `PASS`, findings none.

## Exact reviewed Candidate

```text
loom
3ad7deb7e2210aa305c071690bad24b50aa22984a13df6215d8ee8ea537c919f

loomd
ff5a00dc59cdd1db39998d7013e0989b388da403bb9d4307400e0eb9fd680357

LoomLocalApp
f473684cd6026cd3fd8b80955b2ac81a40a6222f963a72782dc65fd5b65bc06b

arm64 LC_UUID
CE91F84E-4333-35DB-B493-88FADCBC6EC1

canonical complete bundle manifest
e29c1b6c3720492f57cbab4eade293ac1b3cc208caf15f47cd9d4402424a1cbd
```

## Current original state

The original observer is running with target-process Provider marker count
`0`. Original installed binaries/plist and resident SQLite remain at the
frozen hashes and modes. SQLite integrity is `ok`, Event count is `1`.
Candidate app/run/socket/launcher/native process and Swift cache are absent.
The two historical canary crash reports remain byte-identical. Git staging is
empty.

## Governance decision

The deterministic product defect is repaired and independently accepted.
However, the only native live allowance was consumed by the failed canary.
Neither the prior broad authorization nor any pre-repair activation is reused
as a post-Review replacement invocation.

A future installed native-window canary requires new explicit governance that
reconciles the consumed no-retry boundary, freezes one replacement allowance
against the exact reviewed Candidate above, and retains the exact rollback
transaction. Until then:

```text
P2A-W1: HUMAN_REQUIRED — DETERMINISTIC REPAIR PASS — LIVE LOCKED
P2A-W2: LOCKED
```

No install, Computer Use, live invocation, commit, or P2A-W2 work is
authorized by this gate.
