# S2-W37 Frozen WorkItem Contract

- ID: `S2-W37`
- Title: One-Shot Triggered Prepared Runtime Observation
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W36 local commit `38891c3`
- Corresponds to: `TECH-PLAN.md §13.1, §14 Slice 2, §15.8`,
  ADR-0002, ADR-0003, ADR-0007, and accepted S2-W36
- Frozen branch/head: `codex/loom-platform-slice2` at `38891c3`

## Owned files

Developer-owned:

- `internal/app/runtime_observation_trigger.go`
- `internal/app/runtime_observation_trigger_test.go`
- `.loom-evidence/phase1-slice2/S2-W37/deliverable.md`

Controller-owned:

- `.loom-evidence/phase1-slice2/S2-W37/contract.md`
- `.loom-evidence/phase1-slice2/S2-W37/contract-review.md`
- `docs/CURRENT.md`
- Controller-owned non-historical `PROGRESS.md` changes

All accepted product/test/ADR files remain unchanged. Ownership expansion
requires a recorded amendment and fresh contract Reviewer `PASS`.

## Objective

Add the smallest caller-triggered scheduling port that:

1. awaits exactly one caller-supplied Runtime observation trigger;
2. after the trigger succeeds, invokes one accepted prepared S2-W36 observer
   exactly once; and
3. returns only the exact observer outputs.

This creates a deterministic one-trigger/one-observation boundary without
choosing an interval or implementing a clock. It does not import `time`, create
a timer/ticker/goroutine, loop, retry, load configuration, start a daemon,
rebuild projection, access Journal/SQLite, infer absence, or activate a
Runtime.

## Frozen public boundary

```go
var ErrInvalidTriggeredPreparedRuntimeObservationRun = errors.New(
    "invalid triggered prepared runtime observation run",
)

type RuntimeObservationTrigger interface {
    AwaitRuntimeObservation(context.Context) error
}

func RunTriggeredPreparedRuntimeObservationOnce(
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

Imports are limited to standard-library `context`/`errors` plus accepted
`runtime` and `state`.

## Exact behavior

- Nil context, nil/typed-nil trigger, or nil observer returns all five outputs
  zero plus `ErrInvalidTriggeredPreparedRuntimeObservationRun`.
- Canceled/deadline context propagates exactly before the trigger.
- Call `trigger.AwaitRuntimeObservation(ctx)` exactly once.
- Trigger error propagates exactly with five zero outputs and no observer call.
- Check context after a successful trigger; cancellation/deadline here returns
  five zero outputs without calling the observer.
- Call `observer.RunOnce(ctx)` exactly once after the trigger and context check.
- Observer error propagates exactly with five zero outputs and no retry.
- Check context after a successful observer call; late cancellation/deadline
  returns five zero outputs.
- Success returns the exact five observer outputs.

The trigger is an injected port. This WorkItem neither defines what wakes it
nor assumes time, interval, clock, channel, signal, configuration, or daemon
lifecycle semantics.

## No retained state or implicit recurrence

- Trigger and observer are each called at most once per function invocation.
- The function has no loop, recursion, retry, fallback, timer, queue, cache,
  mutex, counter, scheduler state, or goroutine.
- It retains no trigger, observer, Snapshot, Candidate, Event, error, or result.
- Repeated observations require separate explicit caller invocations.
- No claim is made that caller-supplied trigger/observer dependencies are
  independently concurrency-safe.

## Acceptance criteria

1. Nil/typed-nil inputs and pre-canceled/deadline context return five zero
   outputs without trigger/observer work.
2. Trigger runs exactly once before any factory/probe/projection/committer work.
3. Trigger error and trigger-induced cancellation return exact errors, five
   zero outputs, and no observer work.
4. Empty/unchanged/absence-only, discovery-priority, and status-only observer
   paths return exact S2-W36 results after one trigger.
5. Every S2-W36 downstream failure propagates exactly with five zero outputs;
   trigger count remains one and observer work is not retried.
6. Observer-induced cancellation after a selected write remains five-zero and
   no second trigger/observer call occurs.
7. Trigger, factory/probe, and selected/opposite committer order/counts are
   exact.
8. Returned Snapshot/Candidate/Event accessor mutation isolation and explicit
   caller retry idempotency remain unchanged.
9. Temporary SQLite proof uses real accepted projection/committers plus one
   trigger per explicit invocation to prove discovery-priority then status-only
   exact Events, final rebuilt Runtime facts, and retry idempotency.
10. Product calls only trigger plus S2-W36 `observer.RunOnce`; it adds no direct
    S2-W35/lower-layer, time/timer/ticker/channel/signal/loop/goroutine,
    Journal/SQLite/metadata/rebuild/config/daemon/activation/Slice 3 authority.
11. Existing S2-W21 through S2-W36 and repository behavior remains green.

## Mandatory RED

Before product code, add only
`internal/app/runtime_observation_trigger_test.go`. Focused RED must fail only
on the missing frozen S2-W37 symbols.

Required `TestRunTriggeredPreparedRuntimeObservationOnce...` groups:

1. nil/typed-nil input and pre-context validation;
2. exact trigger-before-observer order and no construction/retention work;
3. trigger error/trigger cancellation five-zero and no observer;
4. none/discovery-priority/status success delegation;
5. direct downstream error matrix with exact trigger/factory/writer counts and
   no retry/fallback;
6. explicit repeated calls, mutation isolation, and real temporary SQLite
   discovery→status chain; and
7. static trigger-once/S2-W36-once imports and no time/loop/goroutine/direct
   lower-layer/Journal/metadata/config/daemon/activation/Slice 3 behavior.

Tests use deterministic fake triggers/factories/probes and temporary SQLite
only. They do not wait on wall-clock time, execute installed Pi, read user
Pi/config state, use credentials/network, start a scheduler/daemon, activate a
Runtime, or perform external actions.

## Deterministic checks

- Focused:
  `go test ./internal/app -run
  'TestRunTriggeredPreparedRuntimeObservationOnce' -count=1`
- App: `go test ./internal/app -count=1`
- Impact:
  `go test ./internal/app ./internal/runtime
  ./internal/runtime/discoveryscan ./internal/state
  ./internal/projection ./internal/journal -count=1`
- Focused race:
  `go test -race ./internal/app -run
  'TestRunTriggeredPreparedRuntimeObservationOnce' -count=50`
- Repository: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Format/diff:
  `gofmt -d internal/app/runtime_observation_trigger.go
  internal/app/runtime_observation_trigger_test.go`;
  `git diff --check`
- Scope: only frozen files, S2-W37 evidence, `docs/CURRENT.md`, and
  Controller-owned non-historical `PROGRESS.md` changes.

## Review and exclusions

Fresh contract and implementation Reviewer `PASS` are mandatory. Up to three
contract-reviewed bounded repairs are allowed. Only after complete PASS may one
exact-scope local atomic commit be created.

No concrete clock/timer/ticker/channel/signal/interval/config implementation,
loop/range/recursion/retry/fallback/goroutine, retained lifecycle state,
projection rebuild/source mutation, direct S2-W35 or lower-layer composition,
Journal/SQLite/Event metadata, sequence allocator, dual write, absence
inference, daemon entry/lifecycle, filesystem/process/network/environment
access, concrete Pi adapter, RuntimeProfile selection,
reservation/activation, Team/Agent/Work/Run/Grant/Bridge, credential, external
action, Phase 2, or Slice 3 behavior.

Push, merge, rebase, reset, release, credentials/config mutation, dependency
installation, external mutation, paid work, daemon/Runtime activation, and
Slice 3 remain prohibited.

VERDICT: CONTRACT_FROZEN
