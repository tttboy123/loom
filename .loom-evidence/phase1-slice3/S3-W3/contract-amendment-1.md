# S3-W3 Contract Amendment 1 — Serializable Run/Grant Races

- Parent baseline: `5517a06`
- Date: `2026-07-26`
- Reason: pre-RED feasibility audit
- API change: none
- Owned-file change: none
- Capability change: none

## Finding

The parent contract correctly requires the Grant mutation to compare the
observed Run head, but overstated the consequence of a concurrent Run
mutation. A Grant transaction does not append to `run/<run_id>`. Therefore:

- if `start`, reclaim, lease extension, or terminal commits first, the Grant
  transaction sees a stale Run head and fails;
- if the Grant transaction commits first, the later Run mutation may still
  compare the unchanged Run head and legally commit.

Both results are serializable and safe. Requiring one failure in the second
ordering would need an unauthorized Run-stream marker or a second writer.

## Amendment

Replace only that impossible race statement and its proof requirement with
the two accepted serial orders. Same Grant-stream contenders, including
issue-versus-issue and authorize-versus-revoke, still require exactly one
winner.

Also make the already-frozen authorization idempotency rule explicit:
`RequestID` is unique per Run across all Grants and generations. Only the
exact same current valid Grant/binding/operation may retry it; cross-Grant,
cross-generation, or different-operation reuse fails closed.

No public symbol, Event type, token rule, projection field, transaction
primitive, owned path, trust boundary, or Slice scope changes.

Fresh independent Amendment Review must return `PASS` before mandatory RED.

VERDICT: PASS
