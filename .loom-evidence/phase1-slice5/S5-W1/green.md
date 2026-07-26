# S5-W1 GREEN

Date: 2026-07-26
Baseline: `006db8c`
Candidate status: `accepted_for_local_atomic_commit`

## Vertical capability

The Candidate now provides one bounded local Team observation surface:

1. Journal reads a transaction-consistent related-stream page from exact
   cursor heads without `ReadAll`, writes, retry, or fabricated position.
2. `GlobalReadView` returns copied Team-filtered WorkItems, approvals, and
   per-Run Grants.
3. `TeamExecutionStream` supplies canonical reconnect cursors, closed
   authoritative milestone mapping, board, Attention, recoverable gaps, and a
   bounded in-memory tentative-output subscription.
4. The coordinator refreshes Projection exactly once after first dispatch and
   after executable rebound, before any resulting task executes.
5. Authorized output reaches the subscriber only after Supervisor validation,
   Grant authorization, bound-stream sequencing, and durable private attempt
   capture. ACK/Result control Frames publish no tentative output.
6. `loom timeline` performs one finite read-only page and emits the exact local
   JSON timeline schema with safe exit codes.

## Focused GREEN

```text
go test ./internal/journal ./internal/projection ./internal/api ./internal/app ./cmd/loom
PASS

go test ./internal/work ./internal/rules ./internal/verification ./internal/supervisor ./internal/runtime/piadapter
PASS
```

The controlled external `package app_test` integration proves:

- exact generation 2 is visible after rebound;
- stale generation 1 is rejected and publishes nothing;
- high-risk acceptance executes one independent verifier with its own
  Runtime, Run, generation, Grant, and Evidence lineage;
- verifier tentative output never reaches the source Team subscription;
- attempt capture already contains the authorized Event frame before client
  delivery;
- a closed subscriber does not fail the Run;
- tentative delta is absent from every Journal payload;
- the private receipt and `EvidenceSubmitted` fact exist;
- source and verifier Run/WorkItem/Evidence records resolve to the same `main`
  attempt lineage, with per-Run Grants cross-validated into the related scope;
  and
- final board/Attention and authoritative reconnect state are rebuildable.

Implementation and independent review closed nine contract-shape defects:
encoded cursor length, non-Event observer handling, subscription position
validation, cross-generation coalescing, cross-stream lineage validation,
parent-bound verifier WorkItem lineage, verifier WorkItem inclusion in the
reconnect scope, canonical time/digest payload validation, and WorkItem
Evidence relation validation.

VERDICT: PASS
