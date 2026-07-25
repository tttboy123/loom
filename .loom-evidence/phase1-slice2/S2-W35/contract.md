# S2-W35 Frozen WorkItem Contract

- ID: `S2-W35`
- Title: One-Shot Projected Configured Runtime Observation Cycle
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W34 local commit `a6eb816`
- Corresponds to: `TECH-PLAN.md §13.1, §14 Slice 2, §15.8`,
  ADR-0002, ADR-0003, ADR-0007, and accepted S2-W34
- Frozen branch/head: `codex/loom-platform-slice2` at `a6eb816`

## Owned files

Developer-owned:

- `internal/app/runtime_observation_projected.go`
- `internal/app/runtime_observation_projected_test.go`
- `.loom-evidence/phase1-slice2/S2-W35/deliverable.md`

Controller-owned:

- `.loom-evidence/phase1-slice2/S2-W35/contract.md`
- `.loom-evidence/phase1-slice2/S2-W35/contract-review.md`
- `docs/CURRENT.md`
- Controller-owned non-historical `PROGRESS.md` changes

All accepted product/test/ADR files remain unchanged. Ownership expansion
requires a recorded amendment and fresh contract Reviewer `PASS`.

## Objective

Add the smallest caller-triggered one-shot application boundary that:

1. obtains one accepted copied Runtime projection Snapshot from a caller-bound
   accepted `*projection.Projection`;
2. delegates that exact Snapshot, caller factories, and committers to accepted
   S2-W34 exactly once; and
3. returns only the exact S2-W34 outputs.

This closes the projection-query boundary deferred by S2-W34. It does not
rebuild projection, access Journal/SQLite directly, prepare metadata, retry,
schedule, load config, start a daemon, infer absence, or activate a Runtime.

## Frozen public boundary

```go
var ErrInvalidProjectedConfiguredRuntimeObservationRun = errors.New(
    "invalid projected configured runtime observation run",
)

func RunProjectedConfiguredRuntimeObservationOnce(
    ctx context.Context,
    factories []discoveryscan.ProbeFactory,
    readModel *projection.Projection,
    discoveryCommitter RuntimeDiscoveryCommitter,
    statusCommitter RuntimeStatusCommitter,
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

## Exact behavior

- Nil context or nil read model returns all five outputs zero plus
  `ErrInvalidProjectedConfiguredRuntimeObservationRun`.
- Canceled/deadline context propagates exactly before reading.
- Call accepted `readModel.Snapshot()` exactly once.
- Check context after the Snapshot read.
- Call accepted `RunConfiguredRuntimeObservationOnce` exactly once with the
  exact copied Snapshot and original factories/committers.
- Propagate S2-W34 errors exactly with all five outputs zero.
- Check context after S2-W34.
- Success returns the exact S2-W34 snapshot/plan/commit Candidates.

The read-model Snapshot is obtained before configured discovery, establishing
the cycle baseline at invocation start. The function neither rebuilds nor
refreshes that baseline and never reads it twice.

S2-W34 remains sole authority for discovery execution and S2-W33 path
selection. Therefore empty/unchanged/absence-only remains `none`, inventory or
mixed change remains discovery-only, status-only remains status-only, stable
identity drift remains an error, and absence never implies offline/deletion.

## No partial result or retained state

- Every error returns all five outputs zero.
- Snapshot and S2-W34 are each called at most once.
- No retry, dual write, or alternate projection read occurs.
- The accepted projection Snapshot and downstream Candidates retain their
  established copying/mutation isolation.
- The function retains no read model, Snapshot, factory, committer, or result.

## Acceptance criteria

1. Nil/canceled/deadline context and nil read model return five zero outputs
   without discovery or commit calls.
2. Snapshot is read exactly once before any factory call.
3. Empty/all-absent, unchanged, and absence-only cycles return exact `none`
   outputs with no commit.
4. Current-only/inventory/mixed cycles select only discovery.
5. Status-only cycles select only status.
6. Invalid projection/identity, configured discovery errors, selected
   dependency/write/result errors, and cancellation propagate exactly with all
   five outputs zero and no retry.
7. Factory/probe/committer order and opposite-path non-use remain exact.
8. Projection Snapshot, discovery snapshot, Candidate, and Event accessor
   mutation remains isolated.
9. Temporary SQLite proof uses a real accepted projection rebuilt from prior
   discovery, then proves projected mixed discovery-priority and projected
   status-only configured cycles with exact Events and idempotent retry.
10. Product calls neither `Rebuild` nor direct Journal/SQLite/writer/metadata
    APIs and adds no scheduler/daemon/config/activation/Slice 3 authority.
11. Existing S2-W21 through S2-W34 and repository behavior remains green.

## Mandatory RED

Before product code, add only
`internal/app/runtime_observation_projected_test.go`. Focused RED must fail only
on the missing frozen S2-W35 symbols.

Required `TestRunProjectedConfiguredRuntimeObservationOnce...` groups:

1. context/read-model validation and exact read-before-factory order;
2. none/discovery-priority/status successful paths;
3. discovery/planning/write/cancellation five-zero propagation and no retry;
4. input/result/accessor mutation and exact caller retry;
5. real temporary SQLite projection→configured discovery→write chain; and
6. static exact-one Snapshot/S2-W34 calls, imports, no rebuild/direct Journal/
   metadata/retry/scheduler/daemon/config/activation/Slice 3 assertions.

Tests use deterministic fake factories/probes and temporary SQLite only. They
do not execute installed Pi, read user Pi/config state, use credentials/network,
start a scheduler/daemon, activate a Runtime, or perform external actions.

## Deterministic checks

- Focused:
  `go test ./internal/app -run
  'TestRunProjectedConfiguredRuntimeObservationOnce' -count=1`
- App: `go test ./internal/app -count=1`
- Impact:
  `go test ./internal/app ./internal/runtime
  ./internal/runtime/discoveryscan ./internal/state
  ./internal/projection ./internal/journal -count=1`
- Focused race:
  `go test -race ./internal/app -run
  'TestRunProjectedConfiguredRuntimeObservationOnce' -count=50`
- Repository: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Format/diff:
  `gofmt -d internal/app/runtime_observation_projected.go
  internal/app/runtime_observation_projected_test.go`;
  `git diff --check`
- Scope: only frozen files, S2-W35 evidence, `docs/CURRENT.md`, and
  Controller-owned non-historical `PROGRESS.md` changes.

## Review and exclusions

Fresh contract and implementation Reviewer `PASS` are mandatory. Up to three
contract-reviewed bounded repairs are allowed. Only after complete PASS may one
exact-scope local atomic commit be created.

No projection rebuild/source mutation, direct Journal/SQLite, Event metadata,
S2-W27 unconditional commit, retry, shared sequence allocator, dual write,
absence inference, scheduler/ticker/sleep/goroutine/daemon/config, filesystem/
process/network/environment access, concrete Pi adapter, RuntimeProfile
selection, reservation/activation, Team/Agent/Work/Run/Grant/Bridge,
credential, external action, Phase 2, or Slice 3 behavior.

Push, merge, rebase, reset, release, credentials/config mutation, dependency
installation, external mutation, paid work, daemon/Runtime activation, and
Slice 3 remain prohibited.

VERDICT: CONTRACT_FROZEN
