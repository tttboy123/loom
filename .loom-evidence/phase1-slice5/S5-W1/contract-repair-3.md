# S5-W1 Contract Repair 3 — Accepted Grant Stream Identity

- Date: `2026-07-26`
- Baseline: `006db8c`
- Scope: one existing-authority identity correction; no new file or product
  behavior

## Trigger

The API pre-implementation scope trace found that the accepted Grant Authority
stores a Run's Grant lifecycle in:

```text
agent-grant/<run_id>
```

This is established by `internal/authorization/authority.go` and its accepted
tests. The S5-W1 contract incorrectly wrote `agent-grant/<grant_id>`.
Implementing that spelling would miss real Grant history and invent a second
stream naming convention.

## Bounded repair

Repair 3 changes only the related-scope bullet:

- include exactly one accepted `agent-grant/<run_id>` stream for every
  referenced source/verifier Run;
- continue to use copied `AgentGrantsForRun(run_id)` records only to
  cross-check Grant identity and binding; and
- never create or infer a Grant-ID-keyed stream.

No API signature, owned file, schema, cursor encoding, numeric bound, Event
mapping, test, authority, dependency, migration, WorkItem, S5-W2, or S5-W3
changes. Journal paging and selective-view GREEN completed before discovery
remain inside their frozen behavior and are unaffected. `internal/api`
production implementation remains unstarted until fresh Contract Repair
Review 4 passes.

VERDICT: PASS
