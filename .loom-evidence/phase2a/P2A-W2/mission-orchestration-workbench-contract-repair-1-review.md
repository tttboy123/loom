# P2A-W2 Mission Orchestration Workbench Contract Repair 1 Re-review

**Date**: 2026-07-30
**Reviewer**: fresh independent read-only Contract Reviewer
**Verdict**: `FAIL`

## Findings

- P0: none.
- P1: one new contract finding.
- P2: none.

## Prior P1

`CLOSED`.

Repair 1 exactly owns:

```text
internal/localipc/protocol.go
internal/localipc/protocol_test.go
```

and adds a causal protocol RED. `internal/localipc/server.go` remains excluded.

## New P1 — Board lifecycle lanes conflict with the authorized Extra Goal

The Product Owner froze these lifecycle columns:

```text
Proposed
Ready
Orchestrating
Review
Complete
```

The repaired contract instead listed `Draft` and an independent `Needs You`
lane. The source Goal explicitly requires `Blocked`, `Retrying` and
`Needs You` to remain card status, Attention filters, Timeline facts or Mission
Room inline requests, not lifecycle columns.

## Required Contract Repair 2

Freeze the exact five authorized lifecycle columns and move `Blocked`,
`Retrying` and `Needs You` to the non-lifecycle presentation roles above.

The Reviewer found no other blocking issue in the prepared-command, Projection
facade, owned-file, Provider regression, native fixture/live sequencing, no-W4
or one-canary/no-retry boundaries.

No file was modified and no product, test or live action was performed by the
Reviewer.
