# S3-W2 Contract Amendment 2 — Restart-Safe Journal Read Surface

- Active parent: S3-W2 parent contract plus Amendment 1
- Trigger: mandatory RED passed; minimal GREEN feasibility review
- Date: `2026-07-26`
- Product repair attempts: unchanged

## Problem

The frozen `work.NewAuthority` receives only `*journal.Store`, while the
accepted Store can read only a caller-known stream. `Authority.Snapshot()` must
rebuild all WorkItem and Run records after a new Authority is constructed over
the same/reopened Journal.

An in-memory ID registry would lose restart safety. A hidden index stream would
add non-contracted authoritative Events and complicate every atomic mutation.
Accessing Store internals would violate package and trust boundaries.

## Frozen addition

Add one read-only Journal surface:

```go
func (*Store) ReadAll(context.Context) ([]Event, error)
```

Rules:

1. It returns every committed Event in deterministic
   `stream_id ASC, seq ASC, id ASC` order.
2. Empty Journal returns a non-nil empty slice.
3. Results and payloads are deeply copied.
4. A pre-canceled context returns the exact context error before query work.
5. Query, scan, row, and context failures return no partial facts.
6. It exposes no database handle, transaction, filter, callback, write,
   subscription, or projection authority.
7. Existing `Append`, `AppendBatch`, `AppendBatchIfStreamHeads`, and
   `ReadStream` behavior remains unchanged.

`Authority.Snapshot()` and operation-local replay use `ReadAll`; commands still
append only through the reviewed multi-stream-head CAS. No registry/index Event
is permitted.

## RED and proof addition

Before implementing `ReadAll`, add direct tests for empty, deterministic
ordering, mutation isolation, cancellation, and restart-safe Authority
reconstruction. Capture a focused compile RED only on the missing method.

The existing mandatory RED remains valid. No marker, public work API, Event
schema, lifecycle, projection algorithm, owned file, verification command,
trust boundary, or exclusion changes.

Fresh independent Contract Review must return `PASS` before this read surface
or further S3-W2 product implementation.

VERDICT: PASS
