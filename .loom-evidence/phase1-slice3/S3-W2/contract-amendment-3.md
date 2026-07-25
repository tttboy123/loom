# S3-W2 Contract Amendment 3 — Separate Capacity Stream with Status-Head CAS

- Active parent: S3-W2 parent contract plus Amendments 1 and 2
- Trigger: projection/runtime-status compatibility analysis during minimal GREEN
- Date: `2026-07-26`
- Product repair attempts: unchanged

## Problem

Amendment 1 placed capacity facts directly in accepted
`runtime_instance:<runtime_instance_id>`. Existing accepted runtime-status
facts bind `previous_sequence` to the previous status-bearing fact and require
the next status Event to be adjacent. Interleaving a capacity Event makes a
later legitimate status transition non-adjacent and causes the accepted
projection and StateWriter to reject it.

Changing the accepted Runtime status writer is outside S3-W2 owned files and
would reopen a closed Slice 2 authority.

## Replacement

Capacity facts use:

```text
runtime_capacity:<runtime_instance_id>
```

The accepted status/discovery stream remains exclusively:

```text
runtime_instance:<runtime_instance_id>
```

Every claim, reclaim, start, and terminal operation reads and includes an exact
head expectation for the accepted Runtime status stream. Every operation that
reserves or releases capacity also includes the exact capacity-stream head.

Consequences:

1. A status fact committed before the CAS is observed and can reject the
   operation as unavailable.
2. A status fact racing after the CAS linearization point is ordered after the
   already validated operation.
3. Competing capacity mutations serialize on the capacity stream.
4. Runtime discovery/status sequence and accepted StateWriter behavior remain
   unchanged.
5. Claim/reclaim/terminal still commit Run, WorkItem when applicable, and
   capacity facts in one SQLite transaction.

The two-pass projection indexes both streams and validates each capacity fact
against the referenced Runtime identity/status fact, Run generation, and
operation causation. Missing, extra, duplicate, mismatched, over-capacity, or
orphaned facts still reject.

## Test repair

After fresh Contract Review `PASS`, update the complete S3-W2 tests so
capacity facts are asserted in `runtime_capacity:<id>`, while direct proof also
asserts:

- the status stream contains discovery/status facts only;
- every relevant CAS includes the status-stream head;
- a concurrent status-head change rejects atomically;
- a later accepted status transition remains projectable after capacity
  activity.

Capture a focused repair RED before changing product behavior.

## Unchanged boundary

No public API, owned file, Event payload, lifecycle, acceptance, lease,
generation, terminal, Journal CAS, read surface, verification command, trust
boundary, or exclusion changes. No accepted Runtime status writer or other
Slice 2 file may change.

This amendment replaces only Amendment 1's same-stream placement rule. Fresh
independent Contract Review must return `PASS` before test repair or further
product implementation.

VERDICT: PASS
