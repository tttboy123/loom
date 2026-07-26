# S3-W5 Contract Amendment 4

Status: FROZEN — independent Contract Review 1 PASS.

Date: `2026-07-26`

Parent: `contract.md`

Authority: `../EXIT-CONTRACT-AMENDMENT-6.md`

## Purpose

Adopt only the reviewed repeated-race test-stability boundary from Slice 3 Exit
Contract Amendment 6. S3-W5 product behavior, APIs, acceptance, and the exact
combined race command are unchanged.

## Owned files

This amendment adds only:

- `internal/runtime/piadapter/local_probe_test.go`
- this amendment, its review, and resulting S3-W5 verification evidence

`internal/runtime/piadapter/execution_adapter_test.go` remains within its
existing S3-W5 compatibility ownership.

Only the three exact non-timeout test budgets in Exit Contract Amendment 6 may
change. No assertion, dedicated timeout/cancellation proof, production file,
dependency, or fixture behavior may change.

No test edit may begin before both amendments receive one fresh independent
contract Review `PASS`.

VERDICT: FROZEN
