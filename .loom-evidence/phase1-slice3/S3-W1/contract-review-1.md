# S3-W1 Contract Review 1

- Reviewer: fresh independent read-only Contract Reviewer
- Baseline: `7b1726e`
- Date: `2026-07-25`

## Verdict

`FAIL`

## Blocking finding

The Frame contract ambiguously says “exactly eleven fields” while listing the
eleven metadata/control fields plus `payload`. `TECH-PLAN.md` defines twelve
top-level fields in total. Because missing and unknown fields fail closed, the
count must be exact before RED.

Required correction: state “exactly twelve top-level fields” or “exactly eleven
metadata/control fields plus payload,” and require exact-12 envelope tests.

## Other findings

No other blocker was found. ACK/session semantics are correctly deferred, no
Run/Grant/persistence/process authority is added, the WorkItem is a substantive
untrusted protocol boundary, and the RED/fuzz/race/static proof is adequate.

The Reviewer independently passed the repository, repository-race, vet,
format, diff, codebase-memory absence, and product-authority scope checks.

Nothing is accepted by this review.

VERDICT: FAIL
