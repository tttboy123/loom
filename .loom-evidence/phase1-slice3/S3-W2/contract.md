# S3-W2 Contract — Run Claim, Lease, Terminal, and Projection Authority

- WorkItem: `S3-W2`
- Risk: `STRICT`
- Frozen branch: `codex/loom-platform-slice2`
- Frozen baseline: `c21a8f1`
- Date: `2026-07-25`
- Authority: `TECH-PLAN.md` sections 6, 8, 13.1, 14, and 15
- Depends on: accepted S3-W1 and accepted Slice 1 Journal/projection

## Purpose

Close one authoritative persistence boundary for WorkItem assignment and Run
claim, prepare lease, start, reclaim, and terminal-once state. The Candidate
must serialize the WorkItem, Run, and Runtime capacity facts in one SQLite
transaction and rebuild the accepted read model from Events.

This is one vertical WorkItem. Journal compare-and-append, domain validation,
authority methods, event codecs, and projection changes may not be split into
new WorkItems.

## Owned files

Developer:

- `internal/journal/store.go`
- `internal/journal/journal_test.go`
- `internal/work/run_authority.go`
- `internal/work/run_authority_test.go`
- `internal/projection/projection.go`
- `internal/projection/projection_test.go`
- `internal/projection/run_authority.go`
- `internal/projection/run_authority_test.go`

Controller:

- `.loom-evidence/phase1-slice3/S3-W2/**`
- `docs/CURRENT.md`
- Controller-owned `PROGRESS.md` hunks

All other product, test, migration, protocol, policy, credential, Runtime
adapter, supervisor, evidence-store, root-governance, user-dirty, `.codex/**`,
and `.loom-drafts/**` files are locked.

No migration is required: authoritative facts remain append-only Journal
Events in the accepted `events` table.

## Frozen Journal CAS surface

Add only:

```go
var ErrStreamHeadConflict error

type StreamHeadExpectation struct {
    StreamID string
    Sequence int64
}

func (*Store) AppendBatchIfStreamHeads(
    context.Context,
    []StreamHeadExpectation,
    []Event,
) ([]Event, error)
```

Rules:

1. Expectations are non-empty, unique, mutation-isolated, use valid non-empty
   stream IDs, and have non-negative sequences; at most eight are accepted.
2. Sequence `0` means the stream must be absent. Positive values mean the
   exact current maximum sequence.
3. The method first applies accepted exact-batch idempotency semantics. A
   wholly identical committed retry returns the original facts even though
   stream heads advanced. Partial or conflicting retries retain the accepted
   typed failures.
4. For a new batch, all expected heads and all event preflight rules are
   checked in the same SQLite transaction before any insert.
5. Any head mismatch returns `ErrStreamHeadConflict`, inserts nothing, and
   does not disclose or overwrite the current head.
6. Successful output is deeply copied and ordered exactly as input.
7. Cancellation, query, insert, or commit failure is atomic.
8. Existing `Append` and `AppendBatch` behavior and API remain unchanged.

This generic primitive exposes no SQL transaction handle or callback and
creates no second writer.

## Frozen work authority surface

Package: `work`

```go
var (
    ErrInvalidRunAuthorityInput error
    ErrRunAuthorityConflict error
    ErrRunNotClaimable error
    ErrRunLeaseActive error
    ErrRunLeaseExpired error
    ErrStaleClaimGeneration error
    ErrRunAlreadyTerminal error
    ErrRuntimeUnavailable error
    ErrRuntimeCapacityExhausted error
)

type WorkItemAssignmentInput struct {
    WorkItemID string
    Title string
    RunID string
    AgentInstanceID string
    CorrelationID string
}

type RunClaimInput struct {
    WorkItemID string
    RunID string
    RuntimeInstanceID string
    AgentInstanceID string
    PrepareLeaseDuration time.Duration
    CorrelationID string
}

type RunGenerationInput struct {
    WorkItemID string
    RunID string
    ClaimID string
    ClaimGeneration int64
    RuntimeInstanceID string
    AgentInstanceID string
    CorrelationID string
}

type RunTerminalInput struct {
    RunGenerationInput
    Status string
    Reason string
}

type WorkItemRecord
type RunRecord
type AuthoritySnapshot
type Authority

func NewAuthority(*journal.Store, func() time.Time, io.Reader) (*Authority, error)

func (*Authority) CreateAndAssign(
    context.Context,
    WorkItemAssignmentInput,
) (WorkItemRecord, RunRecord, error)

func (*Authority) Claim(
    context.Context,
    RunClaimInput,
) (WorkItemRecord, RunRecord, error)

func (*Authority) ExtendPrepareLease(
    context.Context,
    RunGenerationInput,
    time.Duration,
) (RunRecord, error)

func (*Authority) Start(
    context.Context,
    RunGenerationInput,
) (WorkItemRecord, RunRecord, error)

func (*Authority) CommitTerminal(
    context.Context,
    RunTerminalInput,
) (WorkItemRecord, RunRecord, error)

func (*Authority) Snapshot(context.Context) (AuthoritySnapshot, error)

func (WorkItemRecord) ID() string
func (WorkItemRecord) Title() string
func (WorkItemRecord) Status() string
func (WorkItemRecord) RunID() string
func (WorkItemRecord) AgentInstanceID() string

func (RunRecord) ID() string
func (RunRecord) WorkItemID() string
func (RunRecord) Phase() string
func (RunRecord) ClaimID() string
func (RunRecord) ClaimGeneration() int64
func (RunRecord) RuntimeInstanceID() string
func (RunRecord) AgentInstanceID() string
func (RunRecord) PrepareLeaseExpiresAt() time.Time
func (RunRecord) TerminalStatus() string
func (RunRecord) TerminalReason() string

func (AuthoritySnapshot) WorkItems() []WorkItemRecord
func (AuthoritySnapshot) Runs() []RunRecord
```

No other exported `internal/work` symbol is permitted.

The injected clock and random reader are captured immutably. Production uses
`time.Now` and `crypto/rand.Reader`; deterministic tests use fixed private
fixtures. Random material creates a canonical lowercase UUID claim ID and is
not a Grant or credential.

## Stream and Event contract

Streams:

- `work-item/<work_item_id>`
- `run/<run_id>`
- accepted `runtime-instance/<runtime_instance_id>`

All payloads are exact JSON objects with no unknown/missing fields. Schema
version is `1`; timestamps are UTC. Event IDs and idempotency keys are
deterministic from command identity and generation; correlation and causation
are preserved.

Required facts:

1. `CreateAndAssign` atomically appends `WorkItemCreated` sequence 1 and
   `WorkItemAssigned` sequence 2 to a new WorkItem stream. Assignment binds one
   Run and AgentInstance. Exact retry returns the same records; conflicting
   reuse fails closed.
2. `Claim` reads the WorkItem, Run, and Runtime streams, validates the exact
   assignment, current Runtime discovery/status, positive capacity, and active
   reservations, then atomically appends:
   - `RunClaimed` to the Run stream; and
   - `RuntimeCapacityReserved` to the Runtime stream.
3. First claim generation is `1`. A non-terminal Run may be reclaimed only
   when its prepare lease is expired; generation increases by exactly one. The
   reclaim transaction first records `RuntimeCapacityReleased` for the stale
   generation, then the new reservation, and records one new `RunClaimed`.
4. `ExtendPrepareLease` requires the exact current claim/generation/runtime/
   agent binding, phase `claimed`, and a non-expired lease. It appends one
   `RunPrepareLeaseExtended`; expiry must move strictly forward.
5. `Start` requires the same exact binding and non-expired prepare lease. It
   appends one `RunStarted`. Prepare leases never supervise a running Run.
6. `CommitTerminal` requires the exact current generation and a claimed or
   running non-terminal Run. Status is exactly `done`, `failed`, or
   `cancelled`; reason is required for failed/cancelled and empty for done. One
   transaction appends:
   - `RunTerminalCommitted` to the Run stream;
   - `WorkItemTerminal` to the WorkItem stream; and
   - `RuntimeCapacityReleased` to the Runtime stream.
7. Terminal commit stops future lease/start/terminal operations. Identical
   retry returns the same terminal records; a different terminal fails with
   `ErrRunAlreadyTerminal`.

Runtime online/capacity is derived only from accepted
`RuntimeInstanceDiscovered` and `RuntimeInstanceStatusChanged` facts plus
capacity reservation/release facts in the same Runtime stream. The authority
does not trust caller-supplied capacity or status.

All new multi-stream mutations use `AppendBatchIfStreamHeads`; no read-then-
unguarded-append path is allowed. Head conflict maps to
`ErrRunAuthorityConflict`; there is no hidden retry.

## Lifecycle and failure rules

```text
WorkItem: ready -> assigned -> running -> done|failed|cancelled
Run:      unclaimed -> claimed -> running -> terminal
```

- Context is checked before clock/random/source work and at the accepted
  Journal boundary.
- Input IDs are bounded opaque IDs; correlations are canonical UUIDs.
- Lease duration is positive and at most five minutes.
- Clock output must be non-zero UTC and is read exactly once per operation.
- Old-generation lease/start/terminal calls return
  `ErrStaleClaimGeneration` with zero outputs and no write.
- Active lease reclaim returns `ErrRunLeaseActive`; expired start/extend returns
  `ErrRunLeaseExpired`.
- Offline/degraded/missing Runtime returns `ErrRuntimeUnavailable`.
- Active reservations at capacity return `ErrRuntimeCapacityExhausted`.
- Every failed operation returns zero new records and leaves Journal facts and
  the last successful snapshot unchanged.
- Returned records/snapshots are immutable, deterministically ordered, and
  concurrency-safe for reads.

This WorkItem does not implement running heartbeat supervision. S3-W4 owns
process liveness and supervisor-generated terminal failure.

## Projection contract

Extend the accepted `projection.Snapshot` with:

```go
Runs map[string]Run

type Run struct {
    ID string
    WorkItemID string
    Phase string
    ClaimID string
    ClaimGeneration int64
    RuntimeInstanceID string
    AgentInstanceID string
    PrepareLeaseExpiresAt time.Time
    TerminalStatus string
    TerminalReason string
}
```

Extend accepted `projection.WorkItem` with:

```go
RunID string
AgentInstanceID string
```

Projection replay must:

- apply assignment, claim/reclaim, lease, start, terminal, and capacity Events
  in canonical stream order;
- reject transition-before-prerequisite, stale/non-monotonic generation,
  backward lease, mismatched bindings, duplicate/conflicting terminal, release
  without matching reservation, and capacity below zero or above the Runtime's
  accepted capacity;
- accept exact repeated immutable facts under existing replay rules;
- preserve the previous Snapshot on any rebuild failure;
- deeply copy all new maps/records.

Existing Mode, Evidence, Team, AgentInstance, Runtime discovery/status, and
saved-Team projection behavior must remain unchanged.

## Mandatory RED

Before any owned product file changes, add all complete tests and capture
failure only on missing frozen S3-W2 symbols/behavior. Required exact markers:

```text
s3_w2_journal_multi_stream_head_cas
s3_w2_create_assign_exact_retry_conflict
s3_w2_claim_runtime_status_capacity_atomicity
s3_w2_lease_extend_expire_reclaim_generation
s3_w2_start_terminal_once_late_generation
s3_w2_projection_rebuild_failure_isolation
s3_w2_concurrency_mutation_fuzz_static
```

## Required proof

1. Journal CAS zero/exact/mismatch heads, exact retry, conflicts, cancellation,
   rollback, ordering, input/output mutation, and concurrent contenders.
2. Exact Event envelope/payload/order for every operation and generation.
3. Full input, clock, randomness, source corruption, status, capacity, lease,
   generation, transition, terminal, head-conflict, and append-failure matrix.
4. Real temporary SQLite race: two Runs compete for capacity one; exactly one
   claim commits and no partial Run/Runtime facts exist.
5. Lease exact-before/exact-at/after boundary, extension, expired reclaim,
   generation increment, old-generation rejection, and capacity release.
6. Terminal exact retry/different retry/late output and WorkItem/Run/Runtime
   atomicity.
7. Rebuild from real Journal, restart/reopen, deterministic ordering, malformed
   fact rejection, and previous-snapshot preservation.
8. Mutation isolation and concurrent reads; fuzz Event payload/state-machine
   replay never panics.
9. Static proof: no process, filesystem workspace, network, goroutine,
   credential, Grant, Provider/model, Bridge payload, scheduler, retry loop,
   Runtime activation, or Slice 4 authority.

## Verification

```text
go test ./internal/journal ./internal/work ./internal/projection -count=1
go test -race ./internal/journal ./internal/work ./internal/projection -count=30
go test ./... -count=1
go test -race ./... -count=1
go vet ./...
go test ./internal/work -run '^$' \
  -fuzz '^FuzzRunAuthorityReplayNeverPanics$' -fuzztime=5s
gofmt -d internal/journal/store.go internal/journal/journal_test.go \
  internal/work/run_authority.go internal/work/run_authority_test.go \
  internal/projection/projection.go internal/projection/projection_test.go \
  internal/projection/run_authority.go \
  internal/projection/run_authority_test.go
git diff --check
```

No dependency audit is required unless a dependency is added, which this
contract forbids.

## Explicit exclusions

S3-W2 creates no AgentGrant, credential/token, workspace, file/process group,
Bridge session, Runtime Adapter, Agent/model execution, scheduler, retry
policy, heartbeat supervisor, Evidence artifact publication, approval,
Verifier, Team DAG, UI, resident daemon activation, or Slice 4 behavior.

It does not use an installed user Runtime, model, credential, ambient
environment, network, or long-running process. Fresh independent Contract
Review must return `PASS` before RED; fresh independent Implementation Review
must return `PASS` before acceptance or local commit.

VERDICT: PASS
