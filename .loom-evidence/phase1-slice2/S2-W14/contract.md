# S2-W14 Frozen WorkItem Contract

- ID: `S2-W14`
- Title: Atomic Event Journal Batch Append
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S1-W2 Journal, accepted S2-W13, and local S2-W13
  commit `89dbff3`
- Corresponds to: `TECH-PLAN.md §5, §6, §14 Slice 2, §15.2, §15.5,
  §15.16, §15.21`, ADR-0002, and ADR-0003
- Frozen branch/head: `codex/loom-platform-slice2` at `89dbff3`

## Owned files

- `internal/journal/store.go`
- `internal/journal/batch_test.go`
- `.loom-evidence/phase1-slice2/S2-W14/deliverable.md`

Existing S1-W2 tests remain authoritative and must stay green. No migration,
projection, Team, Runtime, CLI, or other accepted product/test file is owned.
Any ownership amendment requires a recorded Controller amendment and fresh
contract Reviewer PASS.

## Objective

Extend the accepted append-only Event Journal with one bounded atomic batch
operation:

```go
AppendBatch(ctx context.Context, events []Event) ([]Event, error)
```

The operation validates the complete batch before mutation and then either:

- commits every new Event in one SQLite transaction;
- returns the complete original committed batch for an exact idempotent retry;
  or
- commits nothing and returns a typed error.

This WorkItem provides the transaction primitive a later saved-Team StateWriter
needs to commit TeamInstance and Main AgentInstance facts together. It does not
define those event payloads, create domain resources, update a projection, or
execute anything.

## Frozen input

- batch length must be `1..32`;
- each Event must satisfy the accepted S1-W2 frozen Event contract;
- input order is the returned order;
- idempotency keys must be unique within the batch;
- `(stream_id, seq)` pairs must be unique within the batch;
- mutable input bytes/strings are copied through the accepted Event
  normalization path; and
- context cancellation/deadline is honored before commit.

Empty or oversized batches, invalid Events, duplicate batch idempotency keys,
and duplicate batch stream/sequence pairs fail before any transaction write.

## Atomic and idempotent semantics

Inside one transaction, the implementation classifies the batch against
committed Journal facts:

1. **none exist**: insert every normalized Event and commit once;
2. **all idempotency keys exist with exactly equal immutable Event content**:
   insert nothing, commit/read-complete, and return the original Events in input
   order;
3. **some but not all exact Events exist**: fail with a typed partial-batch
   conflict and write nothing;
4. **any idempotency key exists with different immutable content**: fail with
   accepted `ErrIdempotencyConflict` and write nothing;
5. **any requested stream/sequence is already occupied by a different Event**:
   fail with accepted `ErrSequenceConflict` and write nothing; and
6. **any validation, query, insert, cancellation, commit, or database error**:
   return zero output and never report success.

Classification must be order-independent. A conflict discovered late in the
input cannot leave earlier batch Events committed. The method may return a
wrapped database/context error, but must not translate it into success.

New typed errors:

- `ErrInvalidEventBatch`;
- `ErrEventBatchTooLarge`;
- `ErrDuplicateBatchIdempotencyKey`;
- `ErrDuplicateBatchStreamSequence`; and
- `ErrPartialEventBatchConflict`.

Existing S1-W2 Event, idempotency, and sequence errors remain unchanged.

## Frozen result

On success, the returned slice:

- has exactly the input length;
- preserves input order;
- contains the normalized committed/original immutable Event values;
- is independent of caller input and later mutation; and
- does not expose transaction, SQL, or mutable internal state.

Failure always returns a nil/empty zero result.

## Acceptance criteria

1. A valid single-Event batch is equivalent in persisted facts to accepted
   `Append`.
2. A valid multi-stream/multi-sequence batch commits all Events exactly once in
   one transaction and returns them in input order.
3. Exact whole-batch retry returns the original Events without increasing row
   count.
4. Reordered exact whole-batch retry succeeds and returns original Events in
   the new input order.
5. Mixed exact-existing/new input fails as partial-batch conflict with no new
   rows.
6. Same idempotency key with changed content fails with accepted conflict and
   no writes.
7. Occupied stream/sequence with a different Event fails with accepted conflict
   and no writes.
8. Invalid/empty/oversized/internal-duplicate batches fail before mutation.
9. A late invalid/conflicting Event cannot partially persist earlier Events.
10. Cancelled/deadline contexts, closed database, insert failure, and commit
    failure return zero output and no false success.
11. Concurrent identical batch submissions produce one committed fact set; no
    duplicate or partial facts are committed. A losing caller may receive the
    original batch or a database/context error, but never false success.
12. Input/result mutation does not alter committed immutable facts.
13. Existing `Append`, `ReadStream`, migration, append-only trigger,
    idempotency, concurrency, and projection behavior remains green.
14. No schema migration, Event type widening, state projection, Team/Agent/
    WorkItem/Run/grant behavior, resource creation, network/filesystem/
    environment access, goroutine in production, daemon/CLI/UI, external
    action, or Slice 3 behavior is introduced.

## Mandatory RED tests

The Developer adds `internal/journal/batch_test.go` before changing
`internal/journal/store.go`. RED must fail on missing frozen S2-W14 symbols only.

Required groups:

1. single and multi-Event success with exact persistence/order;
2. exact and reordered idempotent whole-batch retry;
3. partial-existing, idempotency-conflict, and sequence-conflict atomic failure;
4. empty/oversized/invalid/internal-duplicate preflight failures;
5. late conflict/validation, cancellation, closed DB, insert, and commit failure
   with zero output/no partial rows;
6. concurrent identical batch safety;
7. input/result mutation isolation; and
8. static import/schema/scope boundary assertions.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/journal -run 'TestAppendBatch' -count=1`
- Package full: `go test ./internal/journal -count=1`
- Repeated focused race:
  `go test -race ./internal/journal -run 'TestAppendBatch' -count=50`
- Impact: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/journal/store.go internal/journal/batch_test.go` and
  `git diff --check`
- Scope: no migration change; accepted `Append` signature and Event fields
  remain unchanged.

## Explicit exclusions

No migration/schema change, Event contract widening, StateWriter, TeamInstance/
AgentInstance/WorkItem event payload, projection update, resource creation,
Runtime discovery/execution, WorkItem/Run/Evidence/grant, capacity reservation,
workspace, process/model call, Bridge, claim generation, lease, credential,
network/filesystem/environment access, production goroutine, daemon/CLI/UI,
external action, or Slice 3 behavior.

VERDICT: CONTRACT_FROZEN
