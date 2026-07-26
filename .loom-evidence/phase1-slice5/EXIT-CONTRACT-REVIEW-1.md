# Phase 1 Slice 5 Exit Contract Review 1

Date: 2026-07-26
Reviewer: fresh independent read-only Reviewer
Baseline: `006db8c`

## Blocking findings

1. The exit contract named two vertical WorkItems but did not require their
   child contracts to freeze exact owned files, APIs, errors, schemas, IDs,
   numeric bounds, replay rules, CLI shape, fixtures, and amendment behavior.
   The missing admission checklist could turn either WorkItem into an
   unreviewable catch-all.
2. A scalar bounded Journal position was impossible against the current
   per-stream sequence schema. A later Event in a lexically earlier stream
   could be skipped, while `ReadAll` would violate the bounded-page promise.
3. Tentative delivery claimed subscriber slowness could not fail a Run but did
   not reconcile that promise with the accepted synchronous observer path,
   where private attempt capture precedes the observer and observer errors are
   returned.

## Required repair

- add an exact WorkItem admission/sizing checklist and reviewed-amendment gate;
- freeze a bounded Team-related stream-head vector cursor with exact
  validation, inclusion, new-stream, merge, page, and gap semantics; and
- preserve durable private attempt capture while requiring the delivery
  observer to enqueue/coalesce non-blockingly and absorb subscriber state,
  with verifier output still excluded.

VERDICT: FAIL
