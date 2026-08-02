# P2A-W3 Authoritative Acceptance and Recovery Verification

**Date**: 2026-08-03  
**Status**: `SOURCE LOCKED / IMPLEMENTATION REVIEW PENDING`  
**Live**: locked  
**New WorkItem**: none; P2A-W4 does not exist

## Outcome

The causal Pi-005 terminal-aggregation defect is repaired in the frozen
nine-file boundary.

- Work Authority obtains one UTC operation time and derives the final
  Acceptance Decision itself.
- Exact acceptance replay reconstructs the committed decision from the
  committed time before reading a new clock value.
- Coordinator routes accepted/rejected behavior only from the returned
  authoritative Team record.
- Recovery Authority obtains its own operation time, invokes the existing
  Rules policy through a narrow port, and accepts only an exact genuine
  `rules.RecoveryDecision` whose full immutable semantics match replayed Team
  state and that authority time.
- Exact recovery replay precedes a new clock read.
- zero-delay retry/fallback waves use the Authority-recorded `retry_at` as the
  next wave's logical time; positive delays are not fast-forwarded.
- prepared Review and Recovery commands bind versioned time-independent intent
  digests; final timestamp-bearing Authority digests remain Journal facts.
- caller proposal decisions may be zero, stale-time or malformed without
  changing direct Authority output.

No Event/IPC schema, Rules implementation, Journal, Projection, Swift decoder,
Runtime/Provider, credential or native UI source changed in this repair.

## Tests

Focused Authority, Coordinator, prepared Review/Recovery and real product
advancing-clock tests pass. They prove accepted terminal closure, two bounded
Verifier-rejection recoveries, stable intent digests, Authority-later times,
single acceptance batch timestamps, exact replay with an advanced clock,
concurrent recovery reconciliation and zero duplicate terminal facts.

The first unconstrained parallel `go test ./...` run produced four process
fixture timing failures in existing Pi metadata/cancellation tests. Every
failed test passed immediately in isolated reruns. The complete repository was
then run with package concurrency disabled to prevent fixture contention:

```text
go test -count=1 -p=1 ./...                 PASS
go test -count=1 -race -p=1 ./...           PASS
go vet ./...                                PASS
swift test --package-path apps/macos        PASS
swift build -c release --package-path apps/macos  PASS
```

Swift result: 60 XCTest tests passed, one explicitly visual-only preview export
test skipped, and four Swift Testing cases passed.

## Gate

The immutable source lock is
`.loom-evidence/phase2a/P2A-W3/authoritative-acceptance-recovery-source-lock.json`.
No source change is allowed under that lock. A fresh independent Implementation
Reviewer must return PASS before the single final fresh Pi live lineage may be
materialized. No walkthrough, staging or commit is authorized yet.
