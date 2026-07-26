# S5-W2 Implementation Review 1

Reviewer: independent read-only Reviewer

Verdict: `FAIL`

## Findings

1. The canary used sequential coordinator restarts but did not directly prove
   two independent concurrent coordinator/projection callers and a single
   authoritative CAS winner.
2. It rejected a malformed cursor but did not prove rejection of a canonical
   cursor whose recorded Journal head had become stale/conflicting.
3. Restart exact-once counts omitted the frozen test-only external-effect
   marker.
4. The tentative-before-Done and stale-Frame checks read `work_item/` while
   the accepted authority stream prefix is `work-item/`, making those two
   assertions ineffective.

All verification commands passed, but passing tests did not satisfy these
frozen canary clauses.
