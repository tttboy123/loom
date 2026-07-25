# S2-W38 Frozen WorkItem Contract

- ID: `S2-W38`
- Title: Context-Bounded Triggered Runtime Observation Loop
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W37 local commit `9175f94`
- Corresponds to: `TECH-PLAN.md §13.1, §14 Slice 2, §15.8`,
  accepted ADR-0007
- Frozen branch/head: `codex/loom-platform-slice2` at
  `9175f9426f1e153b05fbc6af6e2fbf7b27103b6e`

## Purpose

Add the smallest recurrence boundary required between accepted one-shot
triggered observation and later daemon scheduling. The loop waits on the
injected S2-W37 trigger, runs the accepted prepared observer once for each
successful trigger, and terminates on the first trigger, context, or
observation error.

This WorkItem owns recurrence only. The trigger implementation owns blocking
and backpressure. S2-W38 does not create a clock, timer, ticker, channel,
signal handler, configuration source, daemon entry, goroutine, retry policy, or
Runtime activation.

## Frozen API

New product file:

`internal/app/runtime_observation_loop.go`

```go
func RunPreparedRuntimeObservationLoop(
    ctx context.Context,
    trigger RuntimeObservationTrigger,
    observer *PreparedProjectedRuntimeObserver,
) error
```

There is no new exported type, constructor, error sentinel, result, callback,
configuration, or lifecycle handle. Invalid input and context errors are the
exact errors returned by accepted S2-W37.

## Required behavior

### Exact composition

1. Repeatedly call
   `RunTriggeredPreparedRuntimeObservationOnce(ctx, trigger, observer)`.
2. Discard the five successful return values because the accepted observer has
   already committed the selected authoritative write.
3. After each nil error, begin exactly one next iteration.
4. On the first non-nil error, return that exact error immediately.

The product source must contain exactly one syntactic call to S2-W37. It may
use exactly one `for` statement for recurrence. It must not call S2-W36,
S2-W35, discovery, planning, reconciliation, committers, Journal, projection,
or any lower layer directly.

### Validation and termination

- Nil context, nil or typed-nil trigger, and nil observer delegate once to
  S2-W37 and return its exact
  `ErrInvalidTriggeredPreparedRuntimeObservationRun`.
- A pre-canceled or expired context delegates once to S2-W37 and returns the
  exact context error without trigger or observer work.
- A trigger error terminates the loop immediately; the observer is not called
  for that iteration and there is no retry or fallback.
- Trigger-induced cancellation terminates immediately with the exact context
  error and no observer call for that iteration.
- An observer/downstream error terminates immediately; there is no next trigger
  wait, retry, fallback, or second write.
- Context cancellation after a selected write returns the exact context error
  produced by S2-W37 and terminates.

### Recurrence semantics

- Each successful trigger wait corresponds to exactly one S2-W37 observation.
- N successful iterations followed by one trigger failure yield exactly
  `N+1` trigger awaits and `N` observer attempts.
- N successful observations followed by an observer failure yield exactly
  `N+1` trigger awaits and `N+1` observer attempts.
- The same trigger and prepared observer references are reused; S2-W38 does
  not copy, construct, retain elsewhere, mutate, replace, or expose them.
- A trigger that returns immediately may cause immediate next iterations. Its
  implementation is the only blocking/backpressure authority in this
  WorkItem; concrete time/config policy remains separate.

## Mandatory RED and tests

Mandatory RED adds only:

`internal/app/runtime_observation_loop_test.go`

Focused RED must fail only because
`RunPreparedRuntimeObservationLoop` is missing.

Required `TestRunPreparedRuntimeObservationLoop...` groups:

1. nil/typed-nil input and pre-context delegation with zero trigger/factory
   work;
2. exact repeated trigger→observer ordering across at least three successful
   observations;
3. first and later trigger error/cancellation termination with exact counts;
4. first and later downstream error termination with exact trigger/factory/
   selected/opposite writer counts and no retry/fallback;
5. mutation isolation across repeated calls;
6. a real temporary SQLite discovery→discovery→status chain terminated by an
   injected sentinel trigger error, proving exact Event types/sequences and
   final rebuilt Runtime facts; and
7. static proof of exactly one S2-W37 call, exactly one `for`, allowed imports,
   and all prohibited authority boundaries.

Tests use deterministic fake triggers/factories/probes and temporary SQLite.
They may terminate loops through fake trigger errors or context cancellation.
They must not wait on wall-clock time, execute installed Pi, read user
Pi/config state, use credentials/network, start a scheduler/daemon, activate a
Runtime, or perform external actions.

## Owned files

Product and tests:

- `internal/app/runtime_observation_loop.go`
- `internal/app/runtime_observation_loop_test.go`

Evidence and status:

- `.loom-evidence/phase1-slice2/S2-W38/**`
- `docs/CURRENT.md`
- Controller-owned non-historical `PROGRESS.md` hunks

Any product/test file outside this list requires a written amendment and fresh
contract Reviewer `PASS` before editing.

## Deterministic checks

- Focused:
  `go test ./internal/app -run
  'TestRunPreparedRuntimeObservationLoop' -count=1`
- App: `go test ./internal/app -count=1`
- Impact:
  `go test ./internal/app ./internal/runtime
  ./internal/runtime/discoveryscan ./internal/state
  ./internal/projection ./internal/journal -count=1`
- Focused race:
  `go test -race ./internal/app -run
  'TestRunPreparedRuntimeObservationLoop' -count=50`
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

S2-W38 owns only repeated delegation to S2-W37 and fail-fast termination. It
adds no time source, clock, timer, ticker, channel, signal, sleep, interval,
backoff, retry, goroutine, wait group, mutex, lifecycle state, start/stop
handle, configuration/file/environment/process/network access, daemon/CLI
entry, projection rebuild/source mutation, direct S2-W36 or lower-layer
composition, Journal/SQLite/Event metadata, sequence allocation, dual write,
absence inference, concrete Pi adapter, RuntimeProfile selection,
reservation/activation, Team/Agent/Work/Run/Grant/Bridge, credential, external
action, Phase 2, or Slice 3 behavior.

Concrete scheduling, configuration, daemon wiring, activation, and lifecycle
supervision remain separate later WorkItems.

Push, merge, rebase, reset, release, credentials/config mutation, dependency
installation, external mutation, paid work, daemon/Runtime activation, and
Slice 3 remain prohibited.

Fresh contract and implementation Reviewer `PASS` are mandatory. Up to three
contract-reviewed bounded repairs are allowed. Only after complete PASS may one
exact-scope local atomic commit be created.

VERDICT: CONTRACT_FROZEN
