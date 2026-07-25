# S2-W36 Frozen WorkItem Contract

- ID: `S2-W36`
- Title: Prepared Projected Configured Runtime Observer
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W35 local commit `c8ecc2a`
- Corresponds to: `TECH-PLAN.md §13.1, §14 Slice 2, §15.8`,
  ADR-0002, ADR-0003, ADR-0007, and accepted S2-W35
- Frozen branch/head: `codex/loom-platform-slice2` at `c8ecc2a`

## Owned files

Developer-owned:

- `internal/app/runtime_observer.go`
- `internal/app/runtime_observer_test.go`
- `.loom-evidence/phase1-slice2/S2-W36/deliverable.md`

Controller-owned:

- `.loom-evidence/phase1-slice2/S2-W36/contract.md`
- `.loom-evidence/phase1-slice2/S2-W36/contract-review.md`
- `docs/CURRENT.md`
- Controller-owned non-historical `PROGRESS.md` changes

All accepted product/test/ADR files remain unchanged. Ownership expansion
requires a recorded amendment and fresh contract Reviewer `PASS`.

## Objective

Add the smallest immutable application binding that:

1. binds a copied configured Runtime probe-factory slice, one accepted
   `*projection.Projection`, and the accepted discovery/status committers;
2. exposes a caller-triggered `RunOnce` operation; and
3. delegates each explicit call exactly once to accepted S2-W35.

This creates the stable one-shot observer port that a later scheduling or
daemon WorkItem may call. It does not choose an interval, create a ticker or
goroutine, load configuration, rebuild projection, prepare Event metadata,
retry, start a daemon, infer absence, or activate a Runtime.

## Frozen public boundary

```go
var ErrInvalidPreparedProjectedRuntimeObserver = errors.New(
    "invalid prepared projected runtime observer",
)

type PreparedProjectedRuntimeObserver struct {
    // unexported immutable bindings
}

func NewPreparedProjectedRuntimeObserver(
    factories []discoveryscan.ProbeFactory,
    readModel *projection.Projection,
    discoveryCommitter RuntimeDiscoveryCommitter,
    statusCommitter RuntimeStatusCommitter,
) (*PreparedProjectedRuntimeObserver, error)

func (observer *PreparedProjectedRuntimeObserver) RunOnce(
    ctx context.Context,
) (
    runtime.RuntimeDiscoverySnapshot,
    RuntimeObservationWritePlanCandidate,
    state.RuntimeDiscoveryCommitCandidate,
    runtime.RuntimeStatusReconciliationCandidate,
    state.RuntimeStatusCommitCandidate,
    error,
)
```

Imports are limited to standard library plus accepted `projection`, `runtime`,
`runtime/discoveryscan`, and `state`.

## Exact construction behavior

- Nil read model returns nil plus
  `ErrInvalidPreparedProjectedRuntimeObserver`.
- Empty or nil factory slices are valid and are copied as an empty binding.
- A non-empty factory slice is shallow-copied exactly once. Replacing,
  appending to, or reslicing the caller slice after construction cannot alter
  the observer's bound slice.
- Factory concrete values and committer interfaces are not invoked,
  introspected, or deep-copied during construction.
- Nil or typed-nil committers remain path-scoped S2-W35 inputs: construction
  does not reject them because `none` does not select a writer.
- Construction performs no projection read, probe build/run, Event write,
  goroutine creation, or external action.

## Exact RunOnce behavior

- A nil receiver returns all five outputs zero plus
  `ErrInvalidPreparedProjectedRuntimeObserver`.
- A non-nil receiver calls accepted
  `RunProjectedConfiguredRuntimeObservationOnce` exactly once with the exact
  caller context and bound factories/read model/committers.
- S2-W35 success outputs are returned exactly.
- S2-W35 errors are propagated exactly with all five outputs zero.
- Each call is independent. The observer retains no Snapshot, Candidate,
  Event, error, or prior result, and performs no implicit retry.
- Explicit caller retry is another single S2-W35 invocation and preserves
  accepted idempotency.

S2-W35 remains sole authority for context validation, Snapshot timing,
configured discovery, discovery-priority planning, path-scoped writer
selection, and five-zero error behavior. The observer does not add validation
that would change those accepted semantics.

## Reuse and mutation boundary

- The observer owns only an immutable shallow copy of the factory slice and
  references to accepted dependencies.
- It exposes no binding accessor or mutator.
- It adds no internal mutex, cache, sequence allocator, retry counter, timer,
  lifecycle state, or background work.
- It does not claim that caller-supplied factory or committer implementations
  are independently concurrency-safe.
- Accepted downstream Snapshot/Candidate/Event accessor copying remains
  unchanged.

## Acceptance criteria

1. Nil read model construction fails without invoking any dependency.
2. Nil receiver `RunOnce` returns five zero outputs and the frozen error.
3. Construction copies the factory slice and performs no discovery, projection
   read, commit, or background work.
4. Empty, unchanged, absence-only, discovery-priority, and status-only calls
   return the exact S2-W35 outputs.
5. Nil/typed-nil path-scoped committers retain accepted `none` behavior and
   fail only if their path is selected.
6. Nil/canceled/deadline context and every S2-W35 downstream failure propagate
   exactly with five zero outputs and no retry.
7. Factory/probe/committer order, opposite-path non-use, and Snapshot-before-
   factory order remain exact.
8. Replacing/reslicing/appending the caller factory slice after construction
   cannot change the bound factories.
9. Separate explicit calls do not retain prior results and preserve exact
   idempotent retry behavior.
10. Temporary SQLite proof uses real accepted projection and committers to
    prove prepared discovery-priority then status-only calls, exact Events,
    final rebuilt Runtime facts, and explicit retry idempotency.
11. Product calls only S2-W35 and adds no direct S2-W34/lower-layer,
    Journal/SQLite/writer/metadata/rebuild/scheduler/daemon/config/activation/
    Slice 3 authority.
12. Existing S2-W21 through S2-W35 and repository behavior remains green.

## Mandatory RED

Before product code, add only `internal/app/runtime_observer_test.go`. Focused
RED must fail only on the missing frozen S2-W36 symbols.

Required `TestPreparedProjectedRuntimeObserver...` groups:

1. construction validation, no-side-effect proof, and factory-slice copy;
2. nil receiver and exact five-zero error behavior;
3. none/discovery-priority/status successful delegation;
4. context/downstream failure propagation, no retry, and path-scoped nil
   dependencies;
5. explicit repeated calls, result mutation isolation, and real temporary
   SQLite discovery→status chain; and
6. static S2-W35-only delegation, imports, and no direct lower-layer/rebuild/
   Journal/metadata/retry/scheduler/daemon/config/activation/Slice 3 behavior.

Tests use deterministic fake factories/probes and temporary SQLite only. They
do not execute installed Pi, read user Pi/config state, use credentials/network,
start a scheduler/daemon, activate a Runtime, or perform external actions.

## Deterministic checks

- Focused:
  `go test ./internal/app -run
  'TestPreparedProjectedRuntimeObserver' -count=1`
- App: `go test ./internal/app -count=1`
- Impact:
  `go test ./internal/app ./internal/runtime
  ./internal/runtime/discoveryscan ./internal/state
  ./internal/projection ./internal/journal -count=1`
- Focused race:
  `go test -race ./internal/app -run
  'TestPreparedProjectedRuntimeObserver' -count=50`
- Repository: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Format/diff:
  `gofmt -d internal/app/runtime_observer.go
  internal/app/runtime_observer_test.go`;
  `git diff --check`
- Scope: only frozen files, S2-W36 evidence, `docs/CURRENT.md`, and
  Controller-owned non-historical `PROGRESS.md` changes.

## Review and exclusions

Fresh contract and implementation Reviewer `PASS` are mandatory. Up to three
contract-reviewed bounded repairs are allowed. Only after complete PASS may one
exact-scope local atomic commit be created.

No exported binding fields/accessors, dependency deep copy or introspection,
projection rebuild/source mutation, direct S2-W34 or lower-layer composition,
Journal/SQLite/Event metadata, S2-W27 unconditional commit, retry, shared
sequence allocator, dual write, absence inference, scheduler/ticker/sleep/
goroutine/daemon/config, filesystem/process/network/environment access,
concrete Pi adapter, RuntimeProfile selection, reservation/activation,
Team/Agent/Work/Run/Grant/Bridge, credential, external action, Phase 2, or
Slice 3 behavior.

Push, merge, rebase, reset, release, credentials/config mutation, dependency
installation, external mutation, paid work, daemon/Runtime activation, and
Slice 3 remain prohibited.

VERDICT: CONTRACT_FROZEN
