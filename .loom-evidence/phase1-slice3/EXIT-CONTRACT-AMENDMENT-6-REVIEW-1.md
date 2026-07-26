# Slice 3 Exit Contract Amendment 6 Review 1

Reviewer: independent read-only contract reviewer

Date: 2026-07-26

## Findings

None.

## Evidence

- Amendment 6 records the exact combined race command, the three inherited
  wall-clock fixture deadlines, the absence of race/protocol/product symptoms,
  isolated affected-family PASS, isolated full Pi Adapter `-race -count=10`
  PASS, and the Problem Analysis classification.
- Only `internal/runtime/piadapter/execution_adapter_test.go` and
  `internal/runtime/piadapter/local_probe_test.go` are reopened. No production,
  API, dependency, Runtime behavior, or activation file is reopened.
- The only permitted edits are PID readiness `5s -> 10s`, Supervisor fixture
  profile `3s -> 10s`, and valid local-probe fixture `3s -> 10s`.
- Dedicated metadata timeout, cancellation grace, after-result timeout,
  process-group cleanup, production timeout maxima, fixture semantics, and the
  exact seven-package acceptance command remain unchanged.
- The verification matrix repeats affected families under race, repeats the
  dedicated timeout/cancellation proofs, and then reruns the original combined
  race command unchanged, so the added fixture budget cannot substitute for
  the product checks.

## Freeze recommendation

Freeze Amendment 6 as written.

VERDICT: PASS
