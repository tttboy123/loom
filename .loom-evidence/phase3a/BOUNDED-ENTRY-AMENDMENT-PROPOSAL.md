# Phase 3A Bounded Entry Amendment Proposal

Date: 2026-08-03

Status: `SUPERSEDED BY AUTHORIZED FROZEN ENTRY-AMENDMENT.md`

Name:

```text
Phase 3A Exact Asset Lineage, Native Materialization and Cross-client Journey Boundary
```

## Purpose

Permit the single P3A-W1 contract to close the three Gate 0 entry gaps without
creating P3A-W2 or separate lineage/materializer/journey WorkItems.

## Proposed reopening

The reviewed ADR/Exit Contract may reopen only the minimum shared boundaries
needed for the one vertical path:

1. saved Team exact Skill binding -> ExecutionPlan/dispatch -> Run/Attempt
   immutable asset revision set;
2. Runtime capability catalog/projection/API plus a private per-Run
   materialization adapter and manifest;
3. correlation-only `journey_id` across production client IPC, Daemon logs,
   Journal metadata and Evidence manifest;
4. production native-window and real-PTY shared-root journey harness and
   cleanup evidence.

The exact owned files are not frozen by this proposal. Read-only contract
discovery is now recorded in `ENTRY-AMENDMENT-DISCOVERY.md` and narrows the
shared reopening candidates to:

- saved-Team binding/instantiation, execution plan and Mission compiler;
- Team/Run authority, execution projection and GlobalReadView;
- Runtime catalog/Pi probe plus private Pi materialization adapter;
- P3A application/API/local IPC/daemon/TUI/Swift production surfaces;
- strict correlation-only journey metadata and Daemon structured logging;
- new `internal/assets/**` authority/domain/projection contracts;
- `.loom-evidence/phase3a/**`.

Any broader ownership requires a separately reviewed repair to this amendment.
Journal Store, Evidence Store, Credential Broker, root policy, P2B-W1 and all
excluded later-phase boundaries remain closed unless a focused Contract Review
finding proves the existing API insufficient.

## Frozen intent if authorized

- The asset revision set is canonical, sorted, bounded, digest-bound and copied
  into authoritative Run/Attempt lineage before execution.
- Materialization happens only after capability compatibility and Grant checks,
  into a private per-Run root; repository/user Skills are read-only collision
  sources and never overwritten.
- Journal records lifecycle, exact manifest digest and lineage, not raw Skill
  bytes; immutable bytes live in the Artifact Store.
- `journey_id` is correlation metadata only and cannot authorize, fence,
  deduplicate or alter domain behavior.
- GUI/TUI journey tooling uses production IPC and authority, never direct
  service calls or visual fixtures.
- All of this remains inside P3A-W1 and its final dual-client journey.

## Still excluded

No second Journal/Projection/writer, Scheduler, sandbox, Provider fallback,
Autopilot, Web UI, Marketplace, remote callback, external install, online
Provider, real user Runtime mutation, P3A-W2, push or merge.

## Decision record

The Product Owner explicitly authorized freezing and independently reviewing
this boundary on `2026-08-03`. The normative frozen text is now
`ENTRY-AMENDMENT.md`; this proposal remains historical discovery evidence.

```text
P3A = ENTRY AMENDMENT FROZEN / REVIEW REQUIRED
P3A-W1 = NOT FROZEN
PRODUCT CODE = LOCKED
GUI/TUI JOURNEY = NOT AUTHORIZED
```
