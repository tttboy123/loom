# Phase 3A Gate 1 Combined Contract Review 1

Date: `2026-08-03`

Reviewer: fresh independent read-only Reviewer

Candidate hashes:

- ADR-0013:
  `b16234afbe8df5c114e7fd881932ab2ceeb8c4b6ad9d24646dd8ccd243a44931`;
- Exit Contract:
  `ddba05f803ce33eb18a8ae384e799b6c1c1d7d7311a6c01cc4b55de1492c3b8a`;
- P3A-W1 Contract:
  `12b8c1f2595c16cb7faabe889982182635b45d3d836f0ccc32e139357629fc9c`.

The Reviewer edited no file, staged nothing and ran no product/live/network
action.

## Findings

```text
P0 = 0
P1 = 4
P2 = 0
```

### P1-1: governance files are not W1 product-owned paths

The child contract placed ADR, `docs/CURRENT.md` and Phase 3A governance/evidence
files inside its owned-file list even though the accepted Entry Amendment
restricts W1 product files to its allowlist. Governance Candidate files must be
review/commit evidence outside the product owned-path manifest.

### P1-2: authoritative time source is contradictory

The child contract said every mutation includes `authoritative_time_utc` while
IPC did not define that client field and accepted authorities derive operation
time from an injected authority clock. Time must be service/authority-derived,
excluded from client command bytes and unavailable to GUI/TUI callers.

### P1-3: Agent/Team/WorkPackage bindings lack authority facts

The Goal requires exact revision/digest binding to Agent, Team and WorkPackage,
but the child contract defined only execution-lineage binding. It lacked a
binding Event, command/action, projection schema and authoritative subject
identity validation.

### P1-4: schemas are not exact enough before RED

Event rows used references such as “full Candidate schema”; IPC action payloads
remained prose; canonical stream/Event/idempotency formulas were deferred to
RED; materialization and journey manifests did not freeze exact JSON fields,
optionality and canonical encoding.

## Verdicts

```text
Product/Authority: FAIL
Operational/Trace Governance: FAIL
```

Gate 1 remains closed. Repair must stay inside the same governance Candidate,
receive fresh independent Re-review and return `P0=P1=P2=0` before RED or any
product code.

VERDICT: `FAIL`
