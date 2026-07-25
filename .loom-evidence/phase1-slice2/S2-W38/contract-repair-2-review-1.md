# S2-W38 Contract Repair 2 Fresh Review 1

- WorkItem: `S2-W38`
- Repair contract SHA-256:
  `f19a9b82253c7cd972b9106eac534ac43b30f0a381f37aa9c3b16e33215e4947`
- Frozen head: `9175f9426f1e153b05fbc6af6e2fbf7b27103b6e`
- Reviewer: fresh independent read-only contract Reviewer

## Findings

None.

## Repair closure

Repair 2 removes the separate projection parameter, rejects a nil private
binding, and captures only `observer.readModel`. This matches accepted S2-W36
and closes the stale-view aliasing defect.

The unexported decorator ordering is exact: underlying await, context check,
then captured projection rebuild. The exported function calls S2-W37 exactly
once and performs the second same-projection rebuild only after S2-W37 success.

Every S2-W37/downstream error remains five-zero and makes no claim that no Event
committed. Exact successful path-specific outputs plus error are limited to
failure of the post-S2-W37 rebuild. The no-post-refresh-context-check rule
matches accepted `Projection.Rebuild` cancellation and atomic-swap behavior.

Journal append remains authoritative, projection rebuild remains read-model
synchronization, and sequential-only/no-overlap scheduling remains explicit.
No new ADR or wider StateWriter authority is required.

VERDICT: PASS
