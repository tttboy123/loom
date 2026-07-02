# Request 003: Stable External Verification Surface Contract

status: pending_scan  
owner: Buddys  
target: Loom upstream  
type: generalized capability request, not direct patch

## 1. Problem

For external-project tasks, an inner sandbox or shadow build can report a result that is hard to interpret from the outside:

1. verification may run against a partial environment that differs from the caller repo
2. toy or placeholder tests can appear in the inner run while the caller cares about real repo acceptance
3. a caller cannot always distinguish:
   - "the implementation is bad"
   - "the verification environment was incomplete"
   - "the verification target was the wrong surface"

As a result, external callers build extra wrapper logic to decide whether a failure belongs to product code or to the verification surface itself.

## 2. Why this should be upstream

This is a generic external-runner problem:

- many external projects have their own repo, env, and acceptance surface
- many users need to know whether a verify result came from the executor sandbox or the caller-declared repo surface
- many automation loops need deterministic failure classes without prose interpretation

## 3. Requested capability surface

### A. Verification surface declaration

External tasks should be able to declare which verification surface a stage used:

- executor sandbox
- attached external repo
- caller-specified verifier hook
- no verification surface available

### B. Verification mismatch classification

Loom should expose machine-readable reasons when verify results are not directly acceptance-grade, for example:

- `verify_surface_incomplete`
- `verify_target_missing`
- `verify_used_placeholder_or_generated_tests`
- `verify_passed_non_authoritative_surface`

### C. Result packet clarity

The closeout packet should distinguish:

- implementation failure
- verification-environment failure
- authoritative repo verification failure
- non-authoritative verify pass

## 4. Non-goals

This request does not ask Loom to:

- run downstream acceptance tests automatically for every repo
- hardcode Buddys test files or repo layout
- replace downstream verifier adapters where they are still needed

## 5. Acceptance criteria for Loom intake

Loom should only take this request if it can aim for:

1. explicit verify-surface labeling in run outputs
2. explicit mismatch/failure classes for non-authoritative verification
3. deterministic closeout fields that help external queue runners avoid prose scraping

## 6. Buddys evidence behind this request

This generalized request is grounded in repeated external-project observations:

- shadow-build verification could fail for environment reasons unrelated to the real product defect
- external callers had to promote real-repo overlay verification above inner sandbox results
- final acceptance required coordinator-side reclassification because inner verify results were not authoritative enough

These observations justify a generic Loom capability request, not a Buddys-specific patch.
