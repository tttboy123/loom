# S2-W33 Frozen WorkItem Contract

- ID: `S2-W33`
- Title: One-Shot Discovery-Priority Runtime Observation Write Coordination
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W32 local commit `26bf981`
- Corresponds to: `TECH-PLAN.md §6, §13.1, §14 Slice 2, §15.8`,
  ADR-0002, ADR-0003, ADR-0007, and accepted
  S2-W27/S2-W31/S2-W32
- Frozen branch/head: `codex/loom-platform-slice2` at `26bf981`

## Owned files

Developer-owned:

- `internal/app/runtime_observation_write.go`
- `internal/app/runtime_observation_write_test.go`
- `.loom-evidence/phase1-slice2/S2-W33/deliverable.md`

Controller-owned:

- `.loom-evidence/phase1-slice2/S2-W33/contract.md`
- `.loom-evidence/phase1-slice2/S2-W33/contract-review.md`
- `docs/CURRENT.md`
- Controller-owned non-historical `PROGRESS.md` changes

All accepted Slice 1 and S2-W1 through S2-W32 product/test/ADR files remain
unchanged. Any ownership expansion requires a recorded amendment and fresh
contract Reviewer `PASS`.

## Objective

Add the smallest caller-triggered one-shot coordinator that:

1. computes the exact accepted S2-W32 write plan once from caller-supplied
   copied projection/current observation;
2. returns without a write for `none`;
3. invokes exactly one accepted discovery committer for `discovery`;
4. invokes accepted S2-W31 exactly once for `status`; and
5. returns the exact immutable plan and selected-path Candidates, with every
   non-selected path remaining zero.

This boundary enforces ADR-0007 single-writer precedence. It does not run
discovery, obtain/rebuild projection state, prepare Event metadata, allocate
sequences, retry, schedule, start a daemon, infer absence, or activate a
Runtime.

## Frozen public boundary

Same accepted package: `internal/app`.

```go
var ErrInvalidRuntimeObservationWriteRun = errors.New(
    "invalid runtime observation write run",
)

func RunRuntimeObservationWriteOnce(
    ctx context.Context,
    projected projection.Snapshot,
    current runtime.RuntimeDiscoverySnapshot,
    discoveryCommitter RuntimeDiscoveryCommitter,
    statusCommitter RuntimeStatusCommitter,
) (
    RuntimeObservationWritePlanCandidate,
    state.RuntimeDiscoveryCommitCandidate,
    runtime.RuntimeStatusReconciliationCandidate,
    state.RuntimeStatusCommitCandidate,
    error,
)
```

The product imports only standard library plus accepted
`internal/projection`, `internal/runtime`, and `internal/state`.

## Planning and path-scoped dependencies

- Nil context returns four zero Candidates plus
  `ErrInvalidRuntimeObservationWriteRun`.
- Canceled/deadline context propagates canonically before planning.
- Call accepted
  `PlanRuntimeObservationWrite(ctx, projected, current)` exactly once.
- Planning errors propagate unchanged with four zero Candidates and neither
  committer inspected or called.
- Check context after planning.
- Validate only the dependency selected by the immutable plan:
  - `none` requires neither committer and ignores nil/typed-nil values;
  - `discovery` requires a nonnil/non-typed-nil discovery committer and does
    not inspect or call the status committer;
  - `status` requires a nonnil/non-typed-nil status committer and does not
    inspect or call the discovery committer.
- A missing required dependency returns four zero Candidates plus
  `ErrInvalidRuntimeObservationWriteRun`.

Path-scoped validation prevents an unused adapter from blocking `none` or the
opposite selected path. Planning still validates all caller-supplied state
before any side effect.

## `none` path

For `RuntimeObservationWriteNone`:

1. check context;
2. return the exact S2-W32 plan;
3. return zero discovery, reconciliation, and status-commit Candidates; and
4. make zero committer calls.

Empty, unchanged, and absent-only observations therefore remain successful
no-write cycles. They do not delete a Runtime or fabricate status.

## `discovery` path

For `RuntimeObservationWriteDiscovery`:

1. require the discovery committer;
2. check context;
3. call `CommitRuntimeDiscovery(ctx, current)` exactly once;
4. propagate committer/context error with four zero Candidates;
5. check context after the call;
6. validate the exact result through accepted S2-W27
   `validRuntimeDiscoveryCommitResult(current, commit)`; and
7. return exact plan plus exact discovery commit, with both status Candidates
   zero.

An invalid nil-error commit returns four zero Candidates plus accepted
`ErrRuntimeDiscoveryCommitResultMismatch`. The coordinator never calls S2-W31
on this path, even when `StatusTransitionCount() > 0`.

## `status` path

For `RuntimeObservationWriteStatus`:

1. require the status committer;
2. check context;
3. call accepted
   `RunProjectedRuntimeStatusReconciliationOnce(
   ctx, projected, current, statusCommitter)` exactly once;
4. propagate its exact error with four zero Candidates;
5. check context after return; and
6. return exact plan plus exact reconciliation/status commit, with discovery
   commit zero.

Accepted S2-W31/S2-W29 remain authority for repeated projection validation,
baseline/status provenance, transition construction, committer delegation,
commit-result matching, context, and no-partial behavior. Because an accepted
S2-W32 `status` plan has at least one transition, this path must return a
nonzero accepted status commit.

## No partial result, retry, or dual write

- Every error returns all four Candidates as zero values.
- Each selected dependency is called at most once.
- The coordinator never retries planning or either committer.
- A successful `none` returns only the plan.
- A successful `discovery` returns only plan plus discovery commit.
- A successful `status` returns only plan plus reconciliation/status commit.
- No path can invoke both committers.

The caller-supplied projection/current inputs and all Candidate/Event accessors
remain isolated through accepted copying. The coordinator retains no input,
result, committer, or mutable request state.

## Acceptance criteria

1. Nil/canceled/deadline context fails before planning/commit with all zero
   Candidates.
2. Planning projection/discovery/identity/context errors propagate exactly and
   make zero committer calls.
3. `none` succeeds with exact plan, three zero downstream Candidates, and
   allows both committers to be nil/typed nil.
4. `discovery` requires only discovery committer, calls it once with exact
   current snapshot, validates exact S2-W27 result, and returns status outputs
   zero.
5. A mixed inventory/status observation still calls only discovery and returns
   status outputs zero.
6. `status` requires only status committer, calls S2-W31 once, and returns exact
   plan/reconciliation/status commit with discovery output zero.
7. Missing selected dependency, committer failure, result mismatch, or
   cancellation returns all zero Candidates with exact inspectable error and
   no retry.
8. Opposite-path invalid committer is never inspected or called.
9. Input/Candidate/Event accessor mutation cannot alter returned facts or call
   counts.
10. A deterministic temporary SQLite integration proves:
    prior discovery → projection rebuild → mixed discovery-priority append
    with zero status commit → projection rebuild → status-only append with zero
    discovery commit, with exact Event types/counts and explicit idempotent
    retry where applicable.
11. Existing S2-W20 through S2-W32, app, Runtime, state, projection, Journal,
    and repository behavior remains green.
12. No discovery execution, projection query/rebuild in product, Event
    metadata allocation, direct Journal/SQLite, unified sequence authority,
    dual write, retry, status-from-absence, configuration/file/process/network,
    scheduler/ticker/sleep/goroutine/daemon, RuntimeProfile selection,
    reservation/activation, Team/Agent/Work/Run/Grant/Bridge, credential,
    external action, or Slice 3 behavior is introduced.

## Mandatory RED tests

Before product code, add only
`internal/app/runtime_observation_write_test.go`. Focused RED must fail only
because the frozen error/coordinator symbols are missing.

Required groups named `TestRunRuntimeObservationWriteOnce...`:

1. context and planning error propagation with all zero outputs/calls;
2. none path with nil/typed-nil dependencies and absence proof;
3. discovery/current-only/inventory/mixed exact single-path delegation;
4. status-only exact S2-W31 delegation;
5. missing selected dependency, source error, result mismatch, delayed
   cancellation, zero outputs, and no retry;
6. opposite committer non-inspection, input/result/accessor mutation
   isolation, and explicit exact retry;
7. real temporary SQLite mixed discovery then status-only Event-chain
   integration; and
8. static import/no-discovery-execution/no-projection-query/no-metadata/
   no-dual-write/no-retry/no-scheduler/no-daemon/no-execution-authority/scope
   assertions.

Tests use deterministic accepted Candidates, fakes, and temporary SQLite only.
They do not use S2-W18, installed Pi, user Pi state, credentials, network,
long-running process, scheduler, daemon, model call, Runtime activation, or
external action.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/app -run
  'TestRunRuntimeObservationWriteOnce' -count=1`
- App package: `go test ./internal/app -count=1`
- Impact:
  `go test ./internal/app ./internal/runtime ./internal/state
  ./internal/projection ./internal/journal -count=1`
- Repeated focused race:
  `go test -race ./internal/app -run
  'TestRunRuntimeObservationWriteOnce' -count=50`
- Repository: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/app/runtime_observation_write.go
  internal/app/runtime_observation_write_test.go` and `git diff --check`
- Scope: only frozen files, S2-W33 evidence, `docs/CURRENT.md`, and
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

No discovery execution/probe/factory, projection query/rebuild/SQLite,
Event/payload/metadata preparation or allocation, retry loop, shared sequence
allocator, discovery/status dual-write, absence-based status/deletion,
scheduler/ticker/sleep/goroutine/daemon/config entry, filesystem/process/
network/environment access, installed Runtime lookup, Pi policy,
RuntimeProfile selection, capacity reservation, Runtime activation,
Team/Agent/WorkItem/Run/Evidence/grant/resource mutation, workspace, model,
Bridge, claim, lease, credential, CLI/UI, external action, Phase 2, or Slice 3
behavior.

VERDICT: CONTRACT_FROZEN
