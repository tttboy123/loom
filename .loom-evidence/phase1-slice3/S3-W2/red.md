# S3-W2 Mandatory RED

- Baseline: `c21a8f1`
- Date: `2026-07-26`
- Active contract: parent plus Amendment 1, fresh Review `PASS`
- Product files changed before RED: none

## Marker proof

Each frozen behavior marker appeared exactly once across the complete test
sources:

```text
s3_w2_journal_multi_stream_head_cas 1
s3_w2_create_assign_exact_retry_conflict 1
s3_w2_claim_runtime_status_capacity_atomicity 1
s3_w2_lease_extend_expire_reclaim_generation 1
s3_w2_start_terminal_once_late_generation 1
s3_w2_projection_rebuild_failure_isolation 1
s3_w2_concurrency_mutation_fuzz_static 1
```

## RED command

```text
go test ./internal/journal ./internal/work ./internal/projection -count=1
```

Exit: `1`.

Compilation failed only on the frozen missing S3-W2 surface and behavior:

- `journal.StreamHeadExpectation`
- `journal.ErrStreamHeadConflict`
- `(*journal.Store).AppendBatchIfStreamHeads`
- `work.Authority` and its frozen inputs/records/operations
- `projection.Snapshot.Runs`
- new `projection.WorkItem` assignment fields

There was no syntax, dependency, test-environment, SQLite, or pre-existing
product failure.

VERDICT: RED
