# Phase 2C Repair 10 Independent Review

**Status**: `PASS` (`P0=0`, `P1=0`, `P2=0`)  
**Date**: 2026-08-08  
**Reviewer**: independent read-only Codex Reviewer `019fe270-896d-7843-873a-f9608028fd15`  
**Repository**: `/Users/lune/Documents/Codex/2026-06-18/hermes-openclaw/loom-pi-rebuild`  
**Branch**: `codex/loom-platform-slice2`  
**Baseline HEAD**: `651f156afda37a8e703cbc0396f9f38b7912600b`  
**Contract SHA-256**: `743c465d09ee9b410f9a91db9640cce98d15dad9ea845043bd769ee7e68eeb39`

## Scope

The Reviewer inspected Repair Amendment section 17, the Repair Candidate
boundary, current-state record, `cmd/loomd/product_daemon.go`,
`cmd/loomd/product_daemon_test.go`, and the accepted Runtime discovery,
baseline, reconciliation, prepared committer, projection, and Journal APIs.
The review was read-only; the Reviewer edited, staged, committed, and pushed
nothing.

## Review 1

The first review returned `P1=2`, `P2=1`:

1. The mandatory private-manifest boundary lacked causal rejection tests for
   mode, symlink, exact path, source identity, future time, and Runtime identity.
2. Exact post-commit restart/replay behavior was not characterized at the
   daemon fixture boundary.
3. `source_commit` was only syntactically validated as 40 hexadecimal bytes.

The repair then bound `source_commit` to the frozen Phase 2C baseline HEAD,
switched manifest reading to one `O_NOFOLLOW` descriptor with descriptor-based
mode/owner/size validation, expanded the zero-write rejection matrix, and
defined exact post-commit restart as a verified zero-append replay. The replay
validator checks deterministic Event, idempotency, reconciliation, envelope,
payload, projection, and authoritative-time identity. Drift remains a startup
failure with no Journal write.

## Re-review 2

Re-review 2 returned `P0=0`, `P1=0`, `P2=1`. The remaining advisory finding was
that replay drift checks were implemented but no causal test constructed an
offline projection authored by a different fixture and proved rejection with
byte-identical Journal facts.

The repair added
`TestControlledRuntimeStatusFixtureRejectsUnmatchedOfflineProjectionWithoutWrite`.
It first commits a valid offline fact from a different fixture, invokes the
intended fixture against that projection, requires rejection, and compares the
complete before/after Journal slices.

## Final Re-review 3

Final Re-review 3 returned:

> PASS - no P0/P1/P2 findings.

The Reviewer confirmed that the remaining P2 is closed by the replay validator
at `cmd/loomd/product_daemon.go:3709` and the unmatched-offline causal test at
`cmd/loomd/product_daemon_test.go:8372`.

## Reviewer Verification

The Reviewer ran:

```text
go test ./cmd/loomd -run 'TestControlledRuntimeStatusFixture|TestProductDaemonProjectsControlledRuntimeOfflineBeforeIPC|TestControlledRuntimeOfflineRecoversThroughOrdinaryPiObservation' -count=10
go test -race ./cmd/loomd -run 'TestControlledRuntimeStatusFixture|TestProductDaemonProjectsControlledRuntimeOfflineBeforeIPC|TestControlledRuntimeOfflineRecoversThroughOrdinaryPiObservation' -count=10
go test ./cmd/loomd -run 'TestControlledRuntimeStatusFixtureRejectsUnmatchedOfflineProjectionWithoutWrite' -count=10 -v
```

All passed. The Reviewer did not run the full repository matrix, vet, or Swift
tests; those remain the next lock-bound Controller gate.

## Verdict Boundary

This `PASS` authorizes Repair 10 source-lock regeneration and deterministic
verification only. It does not accept a Journey, WorkItem, Phase, ADR, Release,
or Product Owner result.
