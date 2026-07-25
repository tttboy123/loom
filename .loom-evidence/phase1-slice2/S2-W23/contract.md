# S2-W23 Frozen WorkItem Contract

- ID: `S2-W23`
- Title: Runtime Status Event Writer
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W22 local commit `b0cf75f`
- Corresponds to: `TECH-PLAN.md §6, §13.1, §14 Slice 2`,
  ADR-0002, ADR-0003, and accepted S2-W14/S2-W20/S2-W22
- Frozen branch/head: `codex/loom-platform-slice2` at `b0cf75f`

## Owned files

- `internal/state/runtime_status_writer.go`
- `internal/state/runtime_status_writer_test.go`
- `.loom-evidence/phase1-slice2/S2-W23/deliverable.md`

The Developer owns only these files. Controller-owned contract, review, status,
and documentation evidence remain outside Developer ownership. All accepted
Slice 1 and S2-W1 through S2-W22 product/test files remain unchanged. Any
ownership expansion requires a recorded Controller amendment and fresh contract
Reviewer `PASS`.

## Objective

Add the smallest authoritative StateWriter boundary that atomically records one
non-empty accepted S2-W22 Runtime status reconciliation Candidate:

1. revalidate the Candidate's public immutable facts and digest;
2. require caller metadata to cover every transition exactly once;
3. emit one canonical `RuntimeInstanceStatusChanged` Event per transition in
   the accepted transition order;
4. bind each Event to the complete reconciliation, baseline, discovery, source,
   stable-identity, previous-fact, and status-change facts;
5. append the complete batch once through accepted S2-W14 `AppendBatch`; and
6. accept only an exact immutable appender result before returning a copied,
   digest-bound commit Candidate.

This WorkItem does not run discovery or reconciliation, infer status from
absence, append discovery Events, update the Runtime projection, read the
current stream head, allocate IDs/sequences, schedule scans, start a daemon,
select/reserve/activate a Runtime, or execute any Agent/Runtime/model/process.

## Frozen public boundary

### Appender

The writer reuses the accepted package-level `state.EventBatchAppender` port:

```go
AppendBatch(context.Context, []journal.Event) ([]journal.Event, error)
```

Product code imports the accepted Journal Event contract but no concrete SQLite
store or driver.

### Commit input

```go
type RuntimeStatusEventInput struct {
    RuntimeInstanceID string
    EventID           string
    IdempotencyKey    string
    Seq               int64
}

type RuntimeStatusCommitInput struct {
    ReconciliationID string
    EmittedAt        time.Time
    Events           []RuntimeStatusEventInput
}
```

`ReconciliationID` is the non-empty common correlation identity for this
bounded status observation. `EmittedAt` is non-zero and normalized to UTC.

The Event-input slice is copied. Its length must be `1..32`, exactly equal the
Candidate transition count, and form a bijection over transition Runtime IDs.
Every Event ID and idempotency key is non-empty and unique. Every sequence must
equal exactly that transition's `PreviousSequence + 1`. Duplicate, missing, or
extra Runtime IDs; duplicate Event IDs, idempotency keys, or derived
stream/sequence pairs; nonpositive, non-advancing, or greater-than-next
sequences; and coverage mismatch fail before append.

Event IDs, idempotency keys, sequences, and `ReconciliationID` are
caller-supplied authoritative metadata. The writer does not allocate them or
read the current stream head. Accepted Journal conflict handling remains
authority for idempotency conflicts and whether the exact next stream sequence
is already occupied.

### Source Candidate validation

`CommitRuntimeStatusTransitions(ctx, appender, candidate, input)` rejects:

- nil context or nil/typed-nil appender;
- canceled or expired context before append;
- a zero or unreconciled Candidate;
- empty transitions;
- more than 32 transitions;
- transition count/accessor mismatch;
- a non-lowercase/non-SHA256 baseline, discovery, or Candidate digest;
- unsorted or duplicate RuntimeInstance IDs;
- empty Runtime, device, adapter, source-probe, or previous-Event identity;
- a source digest differing from the Candidate source discovery digest;
- a nonpositive previous sequence;
- an unknown or equal from/to Runtime status; and
- a recomputed Candidate digest mismatch.

The StateWriter recomputes the accepted S2-W22 Candidate digest schema version
`1` from the copied public facts:

```text
version
baseline_digest
source_discovery_digest
transitions
transition_count
```

Because S2-W22 fields are private and its accessors copy, external packages
cannot construct or mutate a nonzero Candidate directly. Revalidation still
fails closed on every accessible semantic field and does not invent new status
or identity rules.

A valid zero-transition S2-W22 Candidate is a successful no-change
reconciliation but is not an Event-writing command. The writer returns
`ErrEmptyRuntimeStatusCommit`, a zero commit Candidate, and makes zero appender
calls.

## Canonical Events

Events are built in accepted S2-W22 transition order, which is ascending by
RuntimeInstance ID. For each transition:

- `ID`: matched caller `EventID`;
- `StreamID`: `"runtime_instance:" + RuntimeInstanceID`;
- `Seq`: matched caller sequence;
- `IdempotencyKey`: matched caller key;
- `Type`: exactly `RuntimeInstanceStatusChanged`;
- `SchemaVersion`: `1`;
- `EmittedAt`: common normalized UTC time;
- `CorrelationID`: common `ReconciliationID`;
- `CausationID`: exact transition `PreviousEventID`; and
- `PayloadJSON`: exact canonical JSON described below.

Each payload contains:

```text
reconciliation_digest
baseline_digest
source_discovery_digest
source_probe_id
runtime_instance_id
device_id
adapter_type
from_status
to_status
previous_event_id
previous_sequence
```

The envelope sequence must equal exactly `previous_sequence + 1`. This prevents
the writer from appending a gap that accepted projection replay cannot rebuild.
The Journal decides whether that exact next stream sequence is already
occupied. `previous_event_id` and `previous_sequence` remain the exact S2-W22
baseline provenance and are not rewritten to imply independently read
current-stream-head authority.

Payloads contain no display name, executable version, capabilities, capacity,
model IDs, executable/search/isolation path, file identity/digest, environment,
stdout/stderr, credential, Provider secret, prompt, session, or user Pi state.

## Append and result validation

The writer calls `AppendBatch` exactly once with a deep copy of the complete
Event batch. Appender errors propagate and return a zero Candidate. The returned
slice must have the same length and exact immutable Event content in the same
order. Nil, short, long, reordered, envelope-mutated, or payload-mutated results
return `ErrRuntimeStatusCommitResultMismatch` and a zero Candidate.

Exact retries and idempotency/sequence conflicts are delegated to the accepted
Journal. The writer never reports success for a partial or altered batch.

## Commit Candidate

`RuntimeStatusCommitCandidate` has private fields and copied accessors:

```go
Committed() bool
SourceReconciliationDigest() string
BaselineDigest() string
SourceDiscoveryDigest() string
Events() []journal.Event
EventCount() int
CommitDigest() string
```

`CommitDigest` is lowercase SHA-256 over a version tag, the three complete
source digests, and the ordered complete committed Events. Identical exact
retries are stable. Every immutable Event field and every source digest is
digest-sensitive.

Frozen sentinel errors:

```go
ErrInvalidRuntimeStatusCommitInput
ErrInvalidRuntimeStatusCommitSource
ErrRuntimeStatusCommitResultMismatch
ErrRuntimeStatusCommitDigestMismatch
ErrEmptyRuntimeStatusCommit
```

All validation, appender-error, result-mismatch, cancellation, and digest
failures return the zero Candidate.

## Acceptance criteria

1. One and multiple transitions produce exact canonical Events in transition
   order and one exact appender call.
2. Candidate public facts and the accepted version-1 Candidate digest are
   revalidated before append.
3. Commit metadata is copied and forms an exact transition-ID bijection with
   unique Event IDs, idempotency keys, and derived stream/sequence pairs.
4. Every sequence equals exactly its transition's previous sequence plus one;
   lower, equal, or greater values fail before append, and the writer neither
   allocates sequences nor reads stream state.
5. Empty and oversized Candidate/input, invalid source/input/context, and every
   coverage/identity/status/digest failure make zero appender calls.
6. The Event envelope, payload, ordering, CausationID, source bindings, and
   non-disclosure boundary are exact.
7. Appender error and every nil/length/order/envelope/payload mismatch return a
   zero Candidate.
8. Candidate/accessor/input/appender-batch/result/payload/source mutation does
   not alter accepted facts.
9. Commit digest is stable for identical exact retries and sensitive to every
   source digest and immutable Event field.
10. Accepted real Journal behavior proves atomic first append, exact retry,
    idempotency conflict, stream-sequence conflict, partial-batch conflict, and
    unchanged row count on failure, including an occupied exact-next sequence
    rejecting a stale or repeated status Candidate.
11. Existing S2-W2 discovery, S2-W20 writer, S2-W21 projection, S2-W22
    reconciliation, Journal, and all repository behavior remains green.
12. No discovery/reconciliation execution, status-from-absence inference,
    discovery Event write, projection/schema/config/file mutation, scheduler,
    daemon, Runtime selection/reservation/activation, process/model/network/
    environment/credential activity, external action, or Slice 3 behavior is
    introduced.

## Mandatory RED tests

The Developer first creates only
`internal/state/runtime_status_writer_test.go`. RED must fail only on missing
frozen S2-W23 writer/input/Candidate/error symbols.

Required groups named `TestCommitRuntimeStatusTransitions...` or
`TestRuntimeStatusCommit...`:

1. exact one/multi-transition Event envelope, payload, order, and binding;
2. nil/typed-nil appender, nil/canceled/deadline context, invalid Candidate,
   empty/oversized Candidate, invalid metadata, duplicates, below/equal/above
   exact-next sequence errors, and coverage mismatch, all proving zero appender
   calls;
3. appender error plus nil/short/long/reordered/mutated-result rejection;
4. Candidate, accessor, input, appender-batch/result, payload, and source
   mutation isolation;
5. Candidate-digest revalidation and complete commit-digest stability/
   sensitivity;
6. accepted Journal real-SQLite exact retry, all conflict/atomicity cases, and
   occupied exact-next rejection of stale/repeated Candidates;
7. static import, non-disclosure, no-discovery/no-reconciliation/no-projection/
   no-execution, and scope assertions.

Tests obtain source Candidates only through deterministic accepted S2-W2 fake
probes plus S2-W22 reconciliation. They do not invoke Pi, S2-W18/S2-W19,
installed Runtime state, network, credentials, package managers, a scheduler,
or a daemon.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/state -run
  'TestCommitRuntimeStatusTransitions|TestRuntimeStatusCommit' -count=1`
- Package/impact:
  `go test ./internal/state ./internal/runtime ./internal/journal
  ./internal/projection -count=1`
- Repeated focused race:
  `go test -race ./internal/state -run
  'TestCommitRuntimeStatusTransitions|TestRuntimeStatusCommit' -count=30`
- Repository: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/state/runtime_status_writer.go
  internal/state/runtime_status_writer_test.go` and `git diff --check`
- Import boundary: product may import only the standard library plus accepted
  `internal/journal` and `internal/runtime`. It must not import concrete SQL,
  projection, Runtime/Pi adapters, process, network, filesystem, config,
  credentials, scheduler, daemon, CLI/UI, or Slice 3 packages.
- Scope: only the frozen product/test, S2-W23 evidence, `docs/CURRENT.md`, and
  Controller-owned non-historical `PROGRESS.md` changes may enter the
  Candidate.

## Explicit exclusions

No discovery or reconciliation invocation, status-from-absence inference,
`RuntimeInstanceDiscovered` append, status projection, projection-table/direct
SQL/schema/config/file write, installed Pi access, executable/search/isolation
path, environment/output/credentials/prompts/sessions/user Pi state,
RuntimeProfile selection, capacity reservation, Runtime activation, Team/
Agent/WorkItem/Run/Evidence/grant/resource mutation, workspace, process, model,
Bridge, claim, lease, scheduler, daemon/CLI/UI, network, filesystem, production
goroutine, external action, push/merge/release, or Slice 3 behavior.

VERDICT: CONTRACT_FROZEN
