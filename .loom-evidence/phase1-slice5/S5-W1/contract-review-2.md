# S5-W1 Independent Contract Repair Review 2

- Reviewer: fresh independent read-only Contract Reviewer
- Baseline: `006db8c`
- Date: `2026-07-26`
- Scope: exact local API surface Repair 1

## Findings

None.

## Review

The Repair 1 API surface is implementable without package cycles.
`internal/api` may depend on `internal/app` for `NodeOutput`,
`internal/journal` for read-only pages, and `internal/projection` through the
repaired `ViewSource`. The accepted `projection.Projection` already satisfies
`Rebuild(context.Context)` plus `GlobalReadView()`.

The immutable value surface is coherent: page/item accessors expose copied
values, nested records remain JSON-bound to frozen wire schemas, and
`TimelineGapError.Unwrap() []error` preserves `errors.Is` for `ErrStreamGap`
plus one specific safe sentinel without exposing database, path, or raw
payload errors.

The CLI dependency shape is coherent with the existing injection pattern.
`timelineInput` plus the finite read-only `timeline` dependency does not alter
existing `route` or `status` behavior.

Repair 1 adds no owned file, authority, writer, migration, dependency, live
surface, S5-W2, or S5-W3 scope. It introduces no contradiction with the frozen
cursor/page/subscription/JSON semantics.

The repaired S5-W1 contract may proceed to mandatory behavioral RED. No
product behavior is accepted by this review.

VERDICT: PASS
