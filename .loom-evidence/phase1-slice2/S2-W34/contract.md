# S2-W34 Frozen WorkItem Contract

- ID: `S2-W34`
- Title: One-Shot Configured Runtime Observation Cycle
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W33 local commit `affd2a6`
- Corresponds to: `TECH-PLAN.md §13.1, §14 Slice 2, §15.8`,
  ADR-0002, ADR-0003, ADR-0007, and accepted S2-W26/S2-W33
- Frozen branch/head: `codex/loom-platform-slice2` at `affd2a6`

## Owned files

Developer-owned:

- `internal/app/runtime_observation_cycle.go`
- `internal/app/runtime_observation_cycle_test.go`
- `.loom-evidence/phase1-slice2/S2-W34/deliverable.md`

Controller-owned:

- `.loom-evidence/phase1-slice2/S2-W34/contract.md`
- `.loom-evidence/phase1-slice2/S2-W34/contract-review.md`
- `docs/CURRENT.md`
- Controller-owned non-historical `PROGRESS.md` changes

All accepted Slice 1 and S2-W1 through S2-W33 product/test/ADR files remain
unchanged. Any ownership expansion requires a recorded amendment and fresh
contract Reviewer `PASS`.

## Objective

Add the smallest caller-triggered one-shot Runtime observation cycle that:

1. executes accepted S2-W26 configured discovery exactly once from
   caller-supplied factories;
2. passes the resulting exact immutable snapshot plus the caller-supplied
   copied projection to accepted S2-W33 exactly once; and
3. returns the exact snapshot and selected S2-W33 outputs only on success.

This is the discovery-execution boundary explicitly deferred by S2-W33. It
does not use the older S2-W27 unconditional discovery-commit path, obtain or
rebuild projection state, prepare Event metadata, retry, schedule, load
configuration, start a daemon, infer absence, or activate a Runtime.

## Frozen public boundary

Same accepted package: `internal/app`.

```go
var ErrInvalidConfiguredRuntimeObservationRun = errors.New(
    "invalid configured runtime observation run",
)

func RunConfiguredRuntimeObservationOnce(
    ctx context.Context,
    factories []discoveryscan.ProbeFactory,
    projected projection.Snapshot,
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

The product imports only standard library plus accepted
`internal/projection`, `internal/runtime`, `internal/runtime/discoveryscan`,
and `internal/state`.

## Validation and exact composition

- Nil context returns five zero Candidates/snapshots plus
  `ErrInvalidConfiguredRuntimeObservationRun`.
- Canceled/deadline context propagates canonically before discovery.
- Call accepted
  `discoveryscan.DiscoverConfiguredRuntimes(ctx, factories)` exactly once.
- S2-W26 remains sole authority for:
  - zero through 32 caller-ordered factories;
  - defensive factory-slice copying;
  - nil/typed-nil factory rejection;
  - exact explicit absence handling;
  - factory build and probe observation order;
  - discovery validation, canonical error wrapping, and immutable snapshot.
- A discovery error returns five zero outputs and neither committer is
  inspected or called.
- Check context after discovery.
- Call accepted
  `RunRuntimeObservationWriteOnce(
  ctx, projected, snapshot, discoveryCommitter, statusCommitter)` exactly once.
- S2-W33 remains sole authority for projection/current validation,
  discovery-priority classification, path-scoped dependency validation,
  selected commit, result matching, and four-zero error behavior.
- A S2-W33/context error returns five zero outputs, including a zero discovery
  snapshot. No partial result escapes.
- Check context after S2-W33.
- Success returns the exact immutable discovery snapshot and exact S2-W33
  outputs.

The cycle does not prevalidate either committer before discovery because the
selected path is not known until the accepted observation and plan exist.
Invalid unused dependencies remain ignored exactly as S2-W33 requires.

## Successful paths

### `none`

An empty/all-absent, unchanged, or absent-only observation:

- returns the valid exact discovery snapshot;
- returns the exact `none` plan;
- returns zero discovery/reconciliation/status commit Candidates; and
- makes zero committer calls.

Absence remains lack of positive observation, never offline/deletion evidence.

### `discovery`

A current-only, new, or non-status inventory change:

- returns the exact discovery snapshot;
- returns the exact `discovery` plan and discovery commit;
- returns both status Candidates zero; and
- invokes only the discovery committer exactly once.

A mixed inventory/status observation still selects only discovery and retains
the status-transition count as plan evidence.

### `status`

A status-only change with unchanged inventory:

- returns the exact discovery snapshot;
- returns the exact `status` plan, reconciliation, and status commit;
- returns the discovery commit zero; and
- invokes only accepted S2-W31/S2-W29 status path exactly once.

## No partial result, retry, or dual write

- Every error returns the discovery snapshot and all four downstream outputs
  as zero values.
- Discovery and S2-W33 are each called at most once per invocation.
- The cycle never retries a factory, probe, plan, or committer.
- No path invokes both committers.
- Explicit caller retry may produce accepted idempotent commit results, but
  retry policy remains outside this function.

The caller-supplied factory slice, copied projection, discovery snapshot, and
all Candidate/Event accessors remain isolated through accepted copying. The
cycle retains no factory, projection, result, committer, or mutable request
state.

## Acceptance criteria

1. Nil/canceled/deadline context fails before discovery with all five outputs
   zero and no factory/committer calls.
2. Invalid, oversized, nil, or typed-nil factories and factory/probe/discovery
   errors propagate exactly with all outputs zero and no committer calls.
3. Empty/all-absent discovery succeeds as `none` with exact valid empty
   snapshot and nil/typed-nil committers.
4. Unchanged and absent-only observations succeed as `none` and never infer
   offline/deletion.
5. Current-only/inventory observations select discovery and invoke only the
   exact discovery committer once.
6. Mixed inventory/status observations still select discovery only.
7. Status-only observations select status and invoke only accepted S2-W33
   status coordination once.
8. Invalid projection/current/identity, missing selected dependency,
   committer failure, result mismatch, or cancellation returns all five
   outputs zero with exact inspectable error and no retry.
9. Opposite-path invalid committer is never inspected or called.
10. Factory/probe order, input copying, snapshot/Candidate/Event accessor
    mutation, and exact caller retry are deterministic and isolated.
11. A deterministic temporary SQLite integration proves:
    prior discovery → projection rebuild → configured mixed discovery-priority
    cycle with zero status commit → projection rebuild → configured status-only
    cycle with zero discovery commit, exact Event types/counts, and explicit
    idempotent retry.
12. Existing S2-W18 through S2-W33, app, Runtime, discoveryscan, state,
    projection, Journal, and repository behavior remains green.
13. No S2-W27 unconditional discovery application path, projection
    query/rebuild in product, Event metadata allocation, direct Journal/SQLite,
    retry, dual write, absence-based status/deletion, scheduler/ticker/sleep/
    goroutine/daemon/config loader, RuntimeProfile selection, reservation/
    activation, Team/Agent/Work/Run/Grant/Bridge, credential, external action,
    or Slice 3 behavior is introduced.

## Mandatory RED tests

Before product code, add only
`internal/app/runtime_observation_cycle_test.go`. Focused RED must fail only
because the frozen error/coordinator symbols are missing.

Required groups named `TestRunConfiguredRuntimeObservationOnce...`:

1. context and S2-W26 discovery/factory error propagation with all-zero
   outputs/calls;
2. empty/all-absent/unchanged/absence-only `none` paths with nil/typed-nil
   dependencies;
3. current-only/inventory/mixed discovery-priority exact delegation;
4. status-only exact S2-W33 delegation;
5. selected dependency/source/result/cancellation failures, all-zero outputs,
   opposite-path non-use, and no retry;
6. factory/probe ordering, slice/projection/snapshot/result/accessor mutation
   isolation, and explicit exact retry;
7. real temporary SQLite configured mixed discovery then status-only Event
   chain; and
8. static import/exact-composition/no-S2-W27/no-projection-query/no-metadata/
   no-dual-write/no-retry/no-scheduler/no-daemon/no-config/no-execution-
   authority/scope assertions.

Tests use deterministic fake factories/probes, accepted Candidates, and
temporary SQLite only. They do not use S2-W18/S2-W19 concrete Pi execution,
installed Pi, user Pi state, credentials, network, long-running process,
scheduler, daemon, model call, Runtime activation, or external action.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/app -run
  'TestRunConfiguredRuntimeObservationOnce' -count=1`
- App package: `go test ./internal/app -count=1`
- Impact:
  `go test ./internal/app ./internal/runtime
  ./internal/runtime/discoveryscan ./internal/state
  ./internal/projection ./internal/journal -count=1`
- Repeated focused race:
  `go test -race ./internal/app -run
  'TestRunConfiguredRuntimeObservationOnce' -count=50`
- Repository: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/app/runtime_observation_cycle.go
  internal/app/runtime_observation_cycle_test.go` and `git diff --check`
- Scope: only frozen files, S2-W34 evidence, `docs/CURRENT.md`, and
  Controller-owned non-historical `PROGRESS.md` changes may enter the
  Candidate.

## Review and commit policy

- Fresh independent contract Reviewer `PASS` is required before mandatory RED.
- Fresh independent implementation Reviewer `PASS` is required after the
  complete strict matrix.
- Up to three bounded contract-reviewed repairs are permitted.
- Only after all checks and reviews pass may the Controller create one
  exact-scope local atomic commit.
- Push, merge, rebase, reset, release, credential/config mutation, dependency
  installation, external mutation, paid remote work, daemon/Runtime activation,
  and Slice 3 work remain prohibited.

## Explicit exclusions

No S2-W27 application coordinator, projection provider/query/rebuild/SQLite,
Event/payload/metadata preparation or allocation, retry loop, shared sequence
allocator, discovery/status dual-write, absence-based status/deletion,
scheduler/ticker/sleep/goroutine/daemon/config file or loader, filesystem/
process/network/environment access in product, concrete Pi adapter, installed
Runtime lookup, RuntimeProfile selection, capacity reservation, Runtime
activation, Team/Agent/WorkItem/Run/Evidence/grant/resource mutation, workspace,
model, Bridge, claim, lease, credential, CLI/UI, external action, Phase 2, or
Slice 3 behavior.

VERDICT: CONTRACT_FROZEN
