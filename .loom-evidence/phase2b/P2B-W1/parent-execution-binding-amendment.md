# P2B-W1 Parent Execution Binding and Closed Replay Amendment

Status: FROZEN FOR REVIEW
Date: 2026-08-03
Parent contract SHA-256: `2bc98c99c301a014d8dabe7491bd53287148c10803537b6de9d420492c22a866`

## Reason

The frozen contract requires `DecideSideTaskHandoff` to validate the exact
parent execution digest, but the frozen proposal/create, input Artifact,
`SideTaskAdmitted`, projection and read/snapshot schemas do not persist that
binding. The digest is not derivable from Journal TeamExecution facts because
the Phase 2A execution digest also binds the preflight/restart launch lineage.
Treating any syntactically valid digest as authoritative would violate the
decision CAS requirement.

## Exact bounded repair

This is part of P2B-W1. It creates no P2B-W2, new writer, Scheduler, database,
retry loop or SQLite migration.

1. Add required lowercase SHA-256 field `parent_execution_digest` to:
   - `propose` and `create` requests and the canonical proposal digest;
   - Side-task input Artifact schema version 2;
   - `SideTaskAdmitted` schema-version-1 payload;
   - authoritative SideTask record and Projection; and
   - `read` result and each snapshot `side_tasks` item.
2. `create` validates the parent Team/WorkItem/Run/generation and binds the
   exact digest supplied by the current Phase 2A execution result. Admission
   exact replay includes the digest. Restart validates Artifact v2 against the
   admitted digest.
3. Every decision requires the same digest. Work Authority rejects any value
   different from the admitted record before operation time or write. The
   effect digest remains bound to parent Team/WorkItem/Run/generation,
   admitted parent execution digest, handoff digest and decision.
4. `CompleteParentHandoffEffect` must find and exactly validate the applicable
   `ParentContinuationAuthorized` or `ParentCancellationRequested` payload
   before validating the existing terminal TeamExecution/Evidence tuple.
5. Projection validates every frozen lifecycle/decision/effect enum, timeout
   bound, required digest and mode-specific field rule. Unknown or malformed
   values fail rebuild and preserve the previous immutable view.
6. The `side_task_handoff` handler maps all internal/unavailable/deadline
   failures to the method's closed code `internal`; it never emits `timeout`,
   `state_unavailable`, or any other code outside the frozen method set.

## Owned-path delta

No owned path expands beyond the parent contract. Existing P2B-W1 Go, Swift,
TUI, tests and `.loom-evidence/phase2b/P2B-W1/**` paths are sufficient.

## Required proof

- stale or substituted parent execution digest is zero-write at Authority;
- exact restart/read/Swift/TUI round-trip preserves the digest;
- non-authorized completion tuple rejects with zero write;
- illegal projection enums/bounds preserve the prior view;
- every side-task handler failure uses only the closed code set; and
- focused, race, full Go, strict Swift, Release/TSAN and offline vertical
  verification remain mandatory before Implementation Review.
