# P2B-W1 Input Artifact Restart Closure Amendment

Status: FROZEN CANDIDATE
Date: 2026-08-03
Parent contract SHA-256: `2bc98c99c301a014d8dabe7491bd53287148c10803537b6de9d420492c22a866`

## Problem

The reviewed contract requires restart to reconstruct an admitted Side-task from
the `SideTaskAdmitted` fact plus its read-verified input Artifact, preserving the
exact request. The frozen admission Event and input Artifact v1 omit
`decision_timeout_seconds`. A crash after admission and before handoff commit
therefore cannot reproduce the requested timeout without guessing. Guessing,
silently substituting 60 seconds, or abandoning the admitted lineage violates
the restart-closed contract.

## Bounded amendment

New P2B writes use Side-task input Artifact schema version 2. Its exact fields
are the v1 fields plus the required JSON integer `decision_timeout_seconds`:

```text
schema_version=2, side_task_id, parent_mission_id, parent_task_id,
parent_run_id, parent_claim_generation, purpose, mode, title,
authorized_request, permission_scopes, proposal_digest,
decision_timeout_seconds, created_at
```

The value is exactly zero for `report_only`; otherwise it is between 60 and
2,592,000 inclusive. It is copied from the already validated create request,
is covered by the Artifact SHA-256, and is read back with exact decoding before
admission. Restart accepts v2 only for P2B-created state and passes the exact
stored value to `CommitSideTaskHandoff` after the deterministic child lineage
reaches accepted terminal Evidence.

No Journal Event or Event field changes. No SQLite migration. No v1 Artifact
has been accepted or committed before this Candidate ships, so there is no
authoritative v1 production state to migrate. A v1 or unknown-version input
Artifact encountered by restart is corrupt or unrecoverable input: recovery
fails closed, the durable lifecycle remains `admitted`, and no authoritative
`human_required` transition is exposed. The daemon must not claim that this
lineage recovered or is healthy until an operator repairs or removes it under
separately reviewed governance. The Artifact is never silently upgraded.

## Owned files and tests

No owned boundary expands. Implementation remains limited to the parent
contract paths, principally:

- `internal/app/local_product_handoff.go`
- `internal/app/local_product_handoff_test.go`
- `cmd/loomd/product_daemon_test.go`
- `.loom-evidence/phase2b/P2B-W1/**`

Mandatory evidence adds a crash-after-admission fixture proving the same
timeout, child execution identity, Run/Evidence lineage and handoff digest are
recovered once, plus fail-closed v1/unknown schema coverage.

## Unchanged boundaries

The Event Journal remains the sole authority. Artifact bytes remain immutable
supporting evidence, not authority. No P2B-W2, new Scheduler, retry loop,
Provider/network action, credential, second Projection or second writer is
introduced. All other P2B-W1 contract bytes remain unchanged.
