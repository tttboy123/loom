# S3-W2 Contract Amendment 1 — Accepted Streams and Lifecycle Ordering

- Parent contract SHA-256:
  `850ab304f077f99a9d9f2cc7cbd1dbd15be17971cd731111d4fd1b2bc1f96993`
- Trigger: Contract Review 1 `FAIL`
- Date: `2026-07-25`
- Product repair attempts: unchanged

This amendment closes exactly the four Contract Review 1 findings.

## 1. Runtime stream replacement

Replace the Runtime stream spelling everywhere with the already accepted:

```text
runtime_instance:<runtime_instance_id>
```

`RuntimeCapacityReserved` and `RuntimeCapacityReleased` are appended to this
same stream. Every claim/reclaim/terminal CAS includes its exact head, thereby
serializing against accepted discovery, status, and other capacity facts.

## 2. Successful Run versus WorkItem acceptance

`RunTerminalInput.Status` accepts exactly:

- `succeeded`
- `failed`
- `cancelled`

For `succeeded`, `Reason` is empty and the atomic batch appends
`WorkItemReadyForReview`, projecting the WorkItem to `ready_for_review`.
For `failed` or `cancelled`, `Reason` is required and the batch appends
`WorkItemTerminal` with the matching terminal status.

S3-W2 never projects a WorkItem to `done`. `verifying` and `done` remain Slice
4 acceptance/Verifier authority. Run terminal status remains `succeeded`,
`failed`, or `cancelled`.

Replace the WorkItem lifecycle shorthand with:

```text
ready -> assigned -> running -> ready_for_review | failed | cancelled
```

## 3. Reclaim boundary

Prepare-lease expiry authorizes reclaim only while the current Run phase is
`claimed` (pre-start preparation). A `running` Run cannot be reclaimed,
extended, or judged dead from its prepare lease. Running heartbeat/process
ownership and supervisor-generated terminal failure remain S3-W4.

Direct tests must prove that expired `claimed` can be reclaimed with generation
increment, while expired `running` rejects reclaim without any release,
reservation, or Run Event.

## 4. Dependency-aware projection replacement

Replace “apply ... in canonical stream order” for S3-W2 facts with a
deterministic two-pass algorithm:

1. Canonicalize immutable Events and validate each stream's contiguous
   sequence exactly as the accepted replay already does.
2. Parse and index all S3-W2 facts without mutating the candidate Snapshot.
   Build Runtime discovery/status identity and WorkItem create/assignment
   prerequisites first.
3. Replay each Run stream in its own sequence order. Each claim/reclaim and
   terminal operation must cross-reference exactly one matching
   Runtime-capacity fact and exactly one matching WorkItem outcome fact when
   required, using Run ID, claim ID, generation, Runtime ID, Agent ID, and
   causation identity. Independently validate Runtime-stream sequence and
   capacity after every reservation/release.
4. Reject missing, extra, duplicate, mismatched, or orphaned cross-stream
   facts. Publish maps only after the entire candidate succeeds.

The algorithm must not rely on lexicographic stream-name order or database row
adjacency. Direct real-SQLite tests must persist rows whose lexical order puts
`run/...` before `work-item/...` and still rebuild valid state; malformed
cross-stream pairs must fail while preserving the previous Snapshot.

## Unchanged boundary

All API names and fields, Journal CAS rules, owned files, remaining Event and
lease semantics, RED markers, proof matrix, verification commands, trust
boundaries, and exclusions remain unchanged. This amendment adds no Grant,
process, Adapter, heartbeat, acceptance, credential, model, or activation
authority.

The active contract is the parent plus this amendment. Fresh independent
Contract Review must return `PASS` before mandatory RED.

VERDICT: PASS
