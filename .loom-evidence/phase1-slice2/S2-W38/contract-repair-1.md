# S2-W38 Contract Repair 1

- ID: `S2-W38`
- Revised title: Projection-Synchronized Triggered Runtime Observation
- Risk: Strict
- Status: `CONTRACT_REPAIR_FROZEN`
- Supersedes:
  `.loom-evidence/phase1-slice2/S2-W38/contract.md`
- Repairs:
  `.loom-evidence/phase1-slice2/S2-W38/controller-contract-check-1.md`
- Depends on: accepted S2-W37 local commit `9175f94`
- Corresponds to: `TECH-PLAN.md §6, §13.1, §14 Slice 2, §15.8`,
  accepted ADR-0007
- Frozen branch/head: `codex/loom-platform-slice2` at
  `9175f9426f1e153b05fbc6af6e2fbf7b27103b6e`

## Repair decision

Recurrence is postponed to a later WorkItem. S2-W38 instead closes the missing
read-model synchronization boundary around exactly one accepted S2-W37
observation.

An unexported trigger decorator first awaits the caller's trigger, then
rebuilds the accepted projection from committed Journal facts. S2-W37 invokes
the prepared observer only after that refresh. After a successful observation,
S2-W38 rebuilds the same projection again so the committed discovery or status
fact is visible before success returns.

Projection rebuild is read-model synchronization, not state authority. Event
Journal and accepted committers remain the only authoritative writers.

## Repaired frozen API

New product file:

`internal/app/runtime_observation_loop.go`

```go
var ErrInvalidProjectionSynchronizedRuntimeObservationRun = errors.New(
    "invalid projection-synchronized runtime observation run",
)

func RunProjectionSynchronizedRuntimeObservationOnce(
    ctx context.Context,
    trigger RuntimeObservationTrigger,
    readModel *projection.Projection,
    observer *PreparedProjectedRuntimeObserver,
) (
    runtime.RuntimeDiscoverySnapshot,
    RuntimeObservationWritePlanCandidate,
    state.RuntimeDiscoveryCommitCandidate,
    runtime.RuntimeStatusReconciliationCandidate,
    state.RuntimeStatusCommitCandidate,
    error,
)
```

The filename remains frozen from the parent contract to avoid expanding owned
scope. There is no loop function, exported decorator, constructor, callback,
configuration, or lifecycle handle.

## Exact composition

The product defines one unexported decorator implementing
`RuntimeObservationTrigger`:

1. call the underlying `trigger.AwaitRuntimeObservation(ctx)` exactly once;
2. on trigger error, return it immediately without projection rebuild;
3. check `ctx.Err()` and return it before rebuild when canceled;
4. call `readModel.Rebuild(ctx)` exactly once and return its exact error.

The exported function:

1. rejects nil context, nil or typed-nil trigger, nil read model, and nil
   observer with five zero outputs plus the new frozen error;
2. rejects a pre-canceled/expired context with five zero outputs plus the exact
   context error and no trigger/rebuild/observer work;
3. constructs one stack-local decorator over the exact trigger/read model;
4. calls S2-W37 exactly once with that decorator and the exact observer;
5. on any S2-W37 error, returns five zero outputs plus the exact error;
6. after S2-W37 success, calls `readModel.Rebuild(ctx)` exactly once;
7. on post-write rebuild or post-rebuild context failure, returns the five
   successful S2-W37 outputs plus the exact error; and
8. otherwise returns the five exact outputs plus nil.

Product source contains exactly one underlying trigger-await call, exactly one
S2-W37 call, and two syntactic projection `Rebuild` calls: one in the decorator
and one after success. It contains no loop or recurrence.

## Failure semantics

- Trigger failure/cancellation: five zero outputs, no rebuild or observer.
- Pre-observation rebuild failure: five zero outputs, no observer or write.
- Observer/downstream failure: five zero outputs, no post-refresh or retry.
- Successful write followed by projection rebuild failure: return the exact
  successful snapshot/plan/commit/reconciliation/status outputs plus the exact
  rebuild error. The Event remains authoritative; the previous projection
  snapshot remains intact under accepted projection semantics.
- Context cancellation after a successful write follows the same transparent
  partial-success rule: exact successful outputs plus the exact context error.
- No failure path retries a trigger, rebuild, observation, or write.

The asymmetric post-commit error contract is mandatory. Zeroing a committed
write would hide authoritative state and create unsafe retry ambiguity.

## Mandatory RED and tests

Mandatory RED adds only:

`internal/app/runtime_observation_loop_test.go`

Focused RED must fail only on the missing repaired frozen error and function
symbols.

Required `TestRunProjectionSynchronizedRuntimeObservationOnce...` groups:

1. nil/typed-nil input and pre-context validation with zero trigger, projection
   source, factory, and writer work;
2. exact trigger→pre-rebuild→observer→write→post-rebuild order and exact call
   counts for none, discovery, and status paths;
3. trigger error/cancellation and pre-rebuild failure with five zero outputs
   and no observer/write;
4. complete direct S2-W37/downstream failure propagation with five zero outputs,
   no post-refresh, exact selected/opposite writer counts, and no retry;
5. post-write rebuild and post-rebuild context failure with exact non-zero
   successful outputs, exact error, preserved prior projection on rebuild
   failure, and no retry;
6. mutation isolation;
7. one real temporary SQLite sequence using the same read model and prepared
   observer across two explicit calls: seed discovery sequence 1, mixed
   inventory/status discovery sequence 2, then status-only sequence 3; prove
   exact Event types/sequences and final rebuilt Runtime facts without any
   test-side rebuild between the two product calls; and
8. static proof of one trigger await, one S2-W37 call, two `Rebuild` calls,
   allowed imports, no loop, and all prohibited authority boundaries.

Tests use deterministic fake triggers/factories/probes, injectable projection
sources, and temporary SQLite only. They do not wait on wall-clock time,
execute installed Pi, read user Pi/config state, use credentials/network,
start a scheduler/daemon, activate a Runtime, or perform external actions.

## Owned files

Product and tests remain:

- `internal/app/runtime_observation_loop.go`
- `internal/app/runtime_observation_loop_test.go`

Evidence and status remain:

- `.loom-evidence/phase1-slice2/S2-W38/**`
- `docs/CURRENT.md`
- Controller-owned non-historical `PROGRESS.md` hunks

Any product/test file outside this list requires another written amendment and
fresh contract Reviewer `PASS` before editing.

## Deterministic checks

- Focused:
  `go test ./internal/app -run
  'TestRunProjectionSynchronizedRuntimeObservationOnce' -count=1`
- App: `go test ./internal/app -count=1`
- Impact:
  `go test ./internal/app ./internal/runtime
  ./internal/runtime/discoveryscan ./internal/state
  ./internal/projection ./internal/journal -count=1`
- Focused race:
  `go test -race ./internal/app -run
  'TestRunProjectionSynchronizedRuntimeObservationOnce' -count=50`
- Repository: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Format/diff:
  `gofmt -d internal/app/runtime_observation_loop.go
  internal/app/runtime_observation_loop_test.go`;
  `git diff --check`
- Scope: only frozen files, S2-W38 evidence, `docs/CURRENT.md`, and
  Controller-owned non-historical `PROGRESS.md` changes.

## Trust boundary and exclusions

S2-W38 owns only trigger-scoped pre-observation refresh, one S2-W37
observation, and post-success read-model refresh. It may call only accepted
`projection.Projection.Rebuild`; it cannot read Journal/SQLite/Event metadata
directly or mutate a projection snapshot/source.

It adds no loop, recurrence, time source, clock, timer, ticker, channel, signal,
sleep, interval, backoff, retry, goroutine, wait group, mutex, retained
lifecycle state, start/stop handle, configuration/file/environment/process/
network access, daemon/CLI entry, direct S2-W36 or lower-layer composition,
Event metadata, sequence allocation, dual write, absence inference, concrete
Pi adapter, RuntimeProfile selection, reservation/activation,
Team/Agent/Work/Run/Grant/Bridge, credential, external action, Phase 2, or
Slice 3 behavior.

Recurrence, concrete scheduling, configuration, daemon wiring, activation, and
lifecycle supervision remain separate later WorkItems.

Push, merge, rebase, reset, release, credentials/config mutation, dependency
installation, external mutation, paid work, daemon/Runtime activation, and
Slice 3 remain prohibited.

Fresh Repair 1 contract and implementation Reviewer `PASS` are mandatory. Up
to three contract-reviewed bounded product repairs are allowed after mandatory
RED. Only after complete PASS may one exact-scope local atomic commit be
created.

VERDICT: CONTRACT_REPAIR_FROZEN
