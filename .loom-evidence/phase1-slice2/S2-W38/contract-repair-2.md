# S2-W38 Contract Repair 2

- ID: `S2-W38`
- Title: Projection-Synchronized Triggered Runtime Observation
- Risk: Strict
- Status: `CONTRACT_REPAIR_FROZEN`
- Supersedes:
  `.loom-evidence/phase1-slice2/S2-W38/contract-repair-1.md`
- Repairs:
  `.loom-evidence/phase1-slice2/S2-W38/contract-repair-1-review-1.md`
- Informed by:
  `.loom-evidence/phase1-slice2/S2-W38/problem-analysis-1.md`
- Depends on: accepted S2-W37 local commit `9175f94`
- Corresponds to: `TECH-PLAN.md §6, §13.1, §14 Slice 2, §15.8`,
  accepted ADR-0007
- Frozen branch/head: `codex/loom-platform-slice2` at
  `9175f9426f1e153b05fbc6af6e2fbf7b27103b6e`

## Repair decision

Recurrence remains postponed. S2-W38 synchronizes only the exact projection
already bound inside the accepted prepared observer. It removes Repair 1's
independent projection parameter and does not add an accessor or constructor
change.

An unexported trigger decorator awaits the underlying trigger, checks context,
and rebuilds the captured bound projection before S2-W37 invokes the observer.
After S2-W37 success, S2-W38 rebuilds that same projection once more so the
committed fact is visible before success returns.

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

The filename remains frozen from the parent contract. There is no projection
parameter, loop function, exported decorator/accessor, constructor change,
callback, configuration, or lifecycle handle.

## Exact validation and composition

The exported function:

1. rejects nil context, nil or typed-nil trigger, nil observer, and a non-nil
   observer whose private `readModel` binding is nil with five zero outputs plus
   the frozen invalid error;
2. rejects a pre-canceled or expired context with five zero outputs plus the
   exact context error and no trigger/rebuild/observer work;
3. captures the exact bound pointer once:
   `readModel := observer.readModel`;
4. constructs one stack-local unexported decorator containing the exact
   underlying trigger and captured read model;
5. calls accepted `RunTriggeredPreparedRuntimeObservationOnce` exactly once
   with that decorator and the exact observer;
6. on any S2-W37 error, returns five zero outputs plus that exact error and does
   not post-refresh;
7. only after S2-W37 returns success, calls the captured
   `readModel.Rebuild(ctx)` exactly once;
8. on post-success rebuild failure, returns the five exact path-specific
   successful S2-W37 outputs plus that exact error; and
9. otherwise returns the five exact outputs plus nil.

The unexported decorator implements `RuntimeObservationTrigger` and:

1. calls the underlying `trigger.AwaitRuntimeObservation(ctx)` exactly once;
2. returns an underlying trigger error immediately without rebuild;
3. checks `ctx.Err()` and returns it before rebuild when canceled; and
4. calls the captured `readModel.Rebuild(ctx)` exactly once and returns its
   exact error.

Product source contains exactly one underlying trigger-await call, exactly one
S2-W37 call, and two syntactic `Rebuild` calls: one in the decorator and one
after S2-W37 success. It contains no loop or recurrence.

## Exact failure boundary

| Failure point | Required outputs |
|---|---|
| Invalid input or pre-context | Five zero |
| Underlying trigger or decorator context | Five zero |
| Pre-observation rebuild | Five zero |
| Any S2-W37/downstream error | Five zero |
| S2-W37 success, then post-success rebuild error | Exact S2-W37 tuple plus error |
| Post-success rebuild succeeds | Exact S2-W37 tuple plus nil |

The exact successful tuple remains path-specific: none, discovery, and status
paths retain their accepted zero Candidates for unselected paths.

There is no separate `ctx.Err()` check after a successful post-refresh.
`Projection.Rebuild` already checks context while acquiring its gate, before
source access, during replay, and before atomic snapshot swap. A later check
would create an ambiguous error after both observation and refresh completed
without preventing cancellation immediately after the check.

An S2-W37 error does not prove that no Event committed. Accepted lower layers
may commit and then detect cancellation while returning five zero outputs.
S2-W38 cannot recover those discarded values, performs no blind retry, and
makes no no-write claim on that path.

## Mandatory RED and tests

Mandatory RED adds only:

`internal/app/runtime_observation_loop_test.go`

Focused RED must fail only on the missing repaired frozen error and function
symbols.

Required `TestRunProjectionSynchronizedRuntimeObservationOnce...` groups:

1. nil/typed-nil input, non-nil zero-value observer, and pre-context validation
   with zero trigger, projection source, factory, and writer work;
2. exact trigger→pre-rebuild→observer→write→post-rebuild order and exact call
   counts for none, discovery, and status paths;
3. trigger error/cancellation and deterministic pre-rebuild failure with five
   zero outputs and no observer/write;
4. complete direct S2-W37/downstream failure propagation with five zero outputs,
   no post-refresh, exact selected/opposite writer counts, no retry, and no
   assertion that an S2-W37 error proves no Event committed;
5. post-success projection replay failure with the exact successful tuple plus
   exact error, authoritative Event retained, previous projection snapshot
   preserved, and no retry;
6. mutation isolation;
7. one real temporary SQLite chain reusing the same read model, observer,
   trigger, and shallow-copied scripted factory binding across two explicit
   calls: seed discovery sequence 1 without initially rebuilding, first mixed
   inventory/status observation writes discovery sequence 2, product
   post-refresh exposes sequence 2, second unchanged-inventory/status-change
   observation writes status sequence 3, and product post-refresh exposes
   sequence 3; no test-side rebuild is allowed between calls; and
8. static proof of one underlying trigger await, one exact S2-W37 call, two
   `Rebuild` calls, allowed imports, no loop, and all prohibited authority
   boundaries.

The real chain must prove exact persisted Event order:

1. `RuntimeInstanceDiscovered`, sequence 1;
2. `RuntimeInstanceDiscovered`, sequence 2;
3. `RuntimeInstanceStatusChanged`, sequence 3.

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

## Trust boundary and remaining limitation

S2-W38 owns only trigger-scoped pre-observation refresh, one exact S2-W37
observation, and post-success refresh of the same captured bound projection.
It may call only accepted `projection.Projection.Rebuild`; it cannot read
Journal/SQLite/Event metadata directly or mutate a projection snapshot/source.

The one-shot provides sequential successful-call freshness. It does not
serialize concurrent calls across the full refresh→observe/write→refresh
cycle. Concurrent scheduling remains prohibited until a later WorkItem either
prevents overlap or freezes a separate serialization contract.

It adds no loop, recurrence, time source, clock, timer, ticker, channel, signal,
sleep, interval, backoff, retry, goroutine, wait group, mutex, retained
lifecycle state, start/stop handle, configuration/file/environment/process/
network access, daemon/CLI entry, direct S2-W36 or lower-layer composition,
Event metadata, sequence allocation, dual write, absence inference, concrete
Pi adapter, RuntimeProfile selection, reservation/activation,
Team/Agent/Work/Run/Grant/Bridge, credential, external action, Phase 2, or
Slice 3 behavior.

No new ADR, StateWriter, Event type, or exported projection authority is
introduced. Recurrence, serialization, concrete scheduling, configuration,
daemon wiring, activation, and lifecycle supervision remain separate later
WorkItems.

Push, merge, rebase, reset, release, credentials/config mutation, dependency
installation, external mutation, paid work, daemon/Runtime activation, and
Slice 3 remain prohibited.

Fresh Repair 2 contract and implementation Reviewer `PASS` are mandatory. Up
to three contract-reviewed bounded product repairs are allowed after mandatory
RED. Only after complete PASS may one exact-scope local atomic commit be
created.

VERDICT: CONTRACT_REPAIR_FROZEN
