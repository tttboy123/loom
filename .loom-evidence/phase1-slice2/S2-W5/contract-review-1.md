# S2-W5 Contract Review 1

- Reviewer: fresh independent read-only contract Reviewer
- Reviewed contract SHA-256:
  `aa1d001090dbe904b0c22c5f6caf79592f7687a9a8a44be97edcff58b02ea4c8`
- Result: blocking findings

## Findings

### Critical: main-only content could become acceptance-ready

The contract required every selected SubAgent to own a task but did not require
at least one SubAgent or one task. A main-only, no-task, no-gap input could
therefore satisfy those clauses vacuously and return `AcceptanceReady=true`.

That conflicts with the product requirement that a Team cannot degrade into a
Main Agent display alias and with the Main Agent no-delivery boundary.

Required bounded repair:

- require at least one and at most two referenced SubAgents;
- require at least one SubAgent-owned task;
- make main-only/no-task content fail with a typed error or remain explicitly
  not ready; and
- add mandatory RED coverage.

### Required: current checkpoint was stale

`docs/CURRENT.md` still named the S2-W4 contract-review gate after S2-W5 had
become the frozen contract. The status surface must name the S2-W5 repair-review
gate.

## Non-blocking conclusions

- Content enrichment correctly precedes Draft acceptance.
- RuntimeProfile, RuntimeInstance, and model validation is implementable through
  accepted S2-W1 and S2-W3 APIs.
- Exact unique Runtime/model-set equality is strict but implementable.
- Bounded free-text capability gaps match the explicit `capability_gap`
  authority and correctly block readiness.
- No new ADR is required.

VERDICT: FAIL
