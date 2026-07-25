# S2-W31 Frozen WorkItem Contract

- ID: `S2-W31`
- Title: Projected Runtime Status Reconciliation Coordination
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W30 local commit `47f225b`
- Corresponds to: `TECH-PLAN.md §6, §13.1, §14 Slice 2, §15.8`,
  ADR-0002, ADR-0003, and accepted S2-W25/S2-W29/S2-W30
- Frozen branch/head: `codex/loom-platform-slice2` at `47f225b`

## Owned files

- `internal/app/runtime_status_projection.go`
- `internal/app/runtime_status_projection_test.go`
- `.loom-evidence/phase1-slice2/S2-W31/deliverable.md`

Slice 1 and S2-W1 through S2-W30 product/test files remain unchanged. Any
ownership expansion requires a recorded Controller amendment and fresh contract
Reviewer `PASS`.

## Objective

Add the smallest application boundary that closes the adapter explicitly
deferred by S2-W25:

1. accept one caller-supplied copied `projection.Snapshot`;
2. build the exact accepted S2-W25 Runtime status baseline once;
3. pass that baseline, one caller-supplied accepted S2-W2 discovery snapshot,
   and one injected accepted S2-W29 committer into S2-W29 exactly once; and
4. return the exact immutable reconciliation and commit Candidates/errors.

This boundary does not query or rebuild the Journal, obtain a live Projection,
run discovery, append discovery Events, prepare status Event metadata, combine
discovery/status write policy, infer status from absence, schedule, start a
daemon, or activate a Runtime.

## Frozen public boundary

```go
var ErrInvalidProjectedRuntimeStatusRun = errors.New(
    "invalid projected runtime status run",
)

func RunProjectedRuntimeStatusReconciliationOnce(
    ctx context.Context,
    projected projection.Snapshot,
    current runtime.RuntimeDiscoverySnapshot,
    committer RuntimeStatusCommitter,
) (
    runtime.RuntimeStatusReconciliationCandidate,
    state.RuntimeStatusCommitCandidate,
    error,
)
```

The product imports only standard library plus accepted `internal/projection`,
`internal/runtime`, and `internal/state`. It does not import Journal/SQLite,
concrete Runtime adapters, discovery/config, filesystem/process/network,
scheduler, daemon, CLI/UI, Team/Work/Run/Grant/Bridge, credentials, or Slice 3
packages.

## Input validation and ordering

- Nil context or nil/typed-nil committer returns two zero Candidates plus
  `ErrInvalidProjectedRuntimeStatusRun`.
- Canceled or expired context is returned canonically before baseline
  construction.
- Dependency validation occurs before projection validation, so an invalid
  committer cannot be hidden by an invalid projection.
- The caller-supplied projection Snapshot and discovery Snapshot are treated as
  untrusted copied inputs and are not retained or mutated.

The coordinator calls accepted
`projection.BuildRuntimeStatusBaselines(projected)` exactly once. It does not
duplicate or weaken S2-W25 map bounds, canonical Runtime conversion,
discovery/status provenance, Event identity, deterministic ordering, or
mutation-isolation validation.

S2-W25 errors propagate unchanged with two zero Candidates and zero committer
calls. Context is checked again after baseline construction and before S2-W29.

## S2-W29 delegation

On a valid baseline, call
`RunObservedRuntimeStatusReconciliationOnce(ctx, baseline, current, committer)`
exactly once and return its exact Candidates/error.

Accepted S2-W29 remains authority for:

- S2-W22 baseline and discovery revalidation;
- stable identity and no-absence-inference semantics;
- zero-transition no-commit behavior;
- exact non-empty committer delegation;
- S2-W23 public commit-result matching;
- context checks; and
- zero Candidates on error.

The S2-W31 coordinator neither retries nor reconstructs either Candidate.

An empty/nil Runtime projection map is a valid nonnil empty S2-W25 baseline.
Current-only observations therefore produce a valid zero-transition
reconciliation and no commit. Absence of projection records never authorizes an
offline transition.

## Write-chain separation

S2-W31 coordinates only the status path against a caller-supplied current
discovery snapshot. It does not invoke S2-W27 or append a
`RuntimeInstanceDiscovered` Event.

This separation is required because discovery and status writers both consume
caller-authoritative per-Runtime stream sequences. A future policy/scheduler
WorkItem must freeze whether a particular observation is a discovery write,
status write, or another governed command. S2-W31 does not silently compose two
writes to the same Runtime stream or allocate their sequences.

## Acceptance criteria

1. Nil/typed-nil committer, nil context, and canceled/expired context fail
   before baseline construction/delegation with zero Candidates.
2. Empty projection plus empty/current-only discovery returns an exact valid
   zero-transition reconciliation and makes zero committer calls.
3. Discovery-only projection records produce exact S2-W25 baselines and exact
   S2-W29 one/multiple transition delegation.
4. Complete status-bearing projection records select their status Event
   provenance through S2-W25 before S2-W29.
5. Invalid/oversized projection, key/identity/core/model/discovery/status/
   sequence/Event-identity defects propagate
   `projection.ErrInvalidRuntimeStatusBaselineProjection` unchanged with zero
   committer calls.
6. Invalid discovery, identity drift, committer failure, result mismatch, and
   context errors retain exact S2-W29 behavior and zero-output rules.
7. Projection maps/records/slices, generated baseline/source Candidates, and
   returned Candidate/accessor data remain mutation-isolated.
8. A real temporary SQLite integration appends accepted prior discovery facts,
   rebuilds a real projection Snapshot, then wires S2-W31 through accepted
   S2-W30/S2-W23 to append exact status facts and prove explicit exact retry.
9. Existing S2-W20 through S2-W30, Journal, state, projection, Runtime, and
   repository behavior remains green.
10. No Journal query/rebuild inside product, direct SQLite, discovery
    execution/write, metadata allocation, discovery/status policy invention,
    status-from-absence inference, scheduler/ticker/sleep/goroutine/daemon,
    process policy, Runtime selection/reservation/activation, Agent/model/
    Team/Work/Run/Grant/Bridge behavior, credential/network/user-state access,
    external action, or Slice 3 behavior is introduced.

## Mandatory RED tests

The Developer first creates only
`internal/app/runtime_status_projection_test.go`. RED must fail on the missing
frozen error and coordinator symbols.

Required groups named
`TestRunProjectedRuntimeStatusReconciliationOnce...`:

1. nil/typed-nil committer and nil/canceled/expired context prevalidation;
2. empty projection/current-only no-commit and no-absence-inference behavior;
3. exact discovery-only and status-bearing projection baseline provenance into
   one/multiple S2-W29 transitions;
4. S2-W25 invalid projection matrix with zero committer calls;
5. S2-W29 discovery/identity/committer/result/context error propagation;
6. projection/baseline/source/Candidate/accessor mutation isolation;
7. real temporary SQLite prior discovery → projection rebuild → S2-W31 →
   S2-W30/S2-W23 exact status append/retry; and
8. static import, no-Journal-query/no-rebuild/no-discovery-write/
   no-metadata-allocation/no-policy/no-scheduler/no-daemon/
   no-execution-authority/scope assertions.

Tests use deterministic accepted Candidates, copied projection fixtures,
in-memory fakes, and temporary SQLite only. They do not invoke S2-W18 or
installed Pi, inspect user Pi state, access credentials/network, start a daemon,
or activate a Runtime.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/app -run
  'TestRunProjectedRuntimeStatusReconciliationOnce' -count=1`
- Package full: `go test ./internal/app -count=1`
- Impact:
  `go test ./internal/app ./internal/runtime ./internal/state
  ./internal/projection ./internal/journal -count=1`
- Repeated focused race:
  `go test -race ./internal/app -run
  'TestRunProjectedRuntimeStatusReconciliationOnce' -count=50`
- Repository: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/app/runtime_status_projection.go
  internal/app/runtime_status_projection_test.go` and `git diff --check`
- Import boundary: product is standard library plus accepted
  `internal/projection`, `internal/runtime`, and `internal/state`; static
  assertions reject Journal/SQLite, concrete adapters, discovery/config,
  file/process/network, scheduler, daemon, CLI/UI, Team/Work/Run/Grant/Bridge,
  credential, or Slice 3 imports.
- Scope: only the frozen product/test files, S2-W31 evidence,
  `docs/CURRENT.md`, and Controller-owned non-historical `PROGRESS.md` changes
  may enter the Candidate.

## Review and commit policy

- Fresh independent contract Reviewer `PASS` is required before mandatory RED.
- Fresh independent implementation Reviewer `PASS` is required after the
  complete matrix.
- Up to three bounded contract-reviewed repairs are permitted.
- Only after all checks and reviews pass may the Controller create one
  exact-scope local atomic commit.
- Push, merge, rebase, reset, release, credential/config mutation, dependency
  installation, external mutation, paid remote work, daemon/Runtime activation,
  and Slice 3 work remain prohibited.

## Explicit exclusions

No product Journal query/rebuild/SQLite access, discovery execution/write,
Event metadata preparation/allocation, discovery/status write-policy
composition, status-from-absence inference, recurring scheduler/ticker/sleep/
goroutine/daemon/config entry, installed Runtime lookup, Pi process-policy
change, RuntimeProfile selection, capacity reservation, Runtime activation,
Team/Agent/WorkItem/Run/Evidence/grant/resource mutation, workspace, process,
model, Bridge, claim, lease, credential, network, filesystem, environment,
CLI/UI, external action, or Slice 3 behavior.

VERDICT: CONTRACT_FROZEN
