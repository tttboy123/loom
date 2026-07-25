# S2-W29 Frozen WorkItem Contract

- ID: `S2-W29`
- Title: One-Shot Observed Runtime Status Commit Coordination
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W28 local commit `9296832`
- Corresponds to: `TECH-PLAN.md §6, §13.1, §14 Slice 2, §15.8`,
  ADR-0002, ADR-0003, and accepted S2-W22/S2-W23/S2-W25
- Frozen branch/head: `codex/loom-platform-slice2` at `9296832`

## Owned files

- `internal/app/runtime_status.go`
- `internal/app/runtime_status_test.go`
- `.loom-evidence/phase1-slice2/S2-W29/deliverable.md`

Slice 1 and S2-W1 through S2-W28 product/test files remain unchanged. Any
ownership expansion requires a recorded Controller amendment and fresh contract
Reviewer `PASS`.

## Objective

Add the smallest one-shot application coordination boundary between accepted
S2-W22 status reconciliation and an injected accepted S2-W23 commit boundary:

1. validate one injected Runtime status committer before reconciliation;
2. reconcile one caller-supplied accepted S2-W25 baseline against one accepted
   S2-W2 discovery snapshot exactly once;
3. return a valid zero-transition reconciliation without calling the committer;
4. otherwise call the committer exactly once with the exact immutable
   reconciliation Candidate;
5. accept only an exact S2-W23 immutable commit Candidate bound to the
   reconciliation, baseline, discovery, and transition count; and
6. return the exact immutable reconciliation and commit Candidates.

This boundary neither builds a baseline from projection nor prepares S2-W23
Event metadata. It is one explicit caller invocation, not a recurring scan. It
does not run discovery, append discovery Events, infer status from absence,
read or mutate projection, allocate IDs/time/sequences, retry, schedule, start a
daemon, reserve capacity, select a RuntimeProfile, or activate a Runtime.

## Frozen public boundary

```go
var (
    ErrInvalidRuntimeStatusRun = errors.New(
        "invalid runtime status run",
    )
    ErrRuntimeStatusCommitResultMismatch = errors.New(
        "runtime status commit result mismatch",
    )
)

type RuntimeStatusCommitter interface {
    CommitRuntimeStatus(
        context.Context,
        runtime.RuntimeStatusReconciliationCandidate,
    ) (state.RuntimeStatusCommitCandidate, error)
}

func RunObservedRuntimeStatusReconciliationOnce(
    ctx context.Context,
    baseline []runtime.RuntimeStatusBaseline,
    snapshot runtime.RuntimeDiscoverySnapshot,
    committer RuntimeStatusCommitter,
) (
    runtime.RuntimeStatusReconciliationCandidate,
    state.RuntimeStatusCommitCandidate,
    error,
)
```

The product file imports only standard library plus accepted
`internal/runtime` and `internal/state` packages. It does not import
projection, Journal/SQLite, a concrete Runtime adapter, configuration,
filesystem/process/network, scheduler, daemon, CLI, UI, Team, Work, Run,
authorization, credentials, or Bridge packages.

## Input and ordering

- Nil context or nil/typed-nil committer returns zero Candidates plus
  `ErrInvalidRuntimeStatusRun`.
- Canceled or expired context is returned canonically before reconciliation.
- The coordinator calls accepted
  `runtime.ReconcileObservedRuntimeStatuses(ctx, baseline, snapshot)` exactly
  once and does not duplicate or weaken its validation.
- Baseline order/copying, projection provenance semantics, discovery
  validation, stable-identity checks, no-absence-inference policy, transition
  ordering, and Candidate digest authority remain with accepted S2-W22/S2-W25.
- Reconciliation errors propagate unchanged, the committer is not called, and
  both returned Candidates are zero.
- Context is checked again after reconciliation and before commit.

## Zero-transition semantics

A valid accepted S2-W22 reconciliation with zero transitions is a successful
no-change result:

- return the exact reconciliation Candidate;
- return a zero `state.RuntimeStatusCommitCandidate`;
- return nil error; and
- make zero committer calls.

The coordinator never calls S2-W23 for an empty Candidate and therefore does
not reinterpret accepted `state.ErrEmptyRuntimeStatusCommit` as an error.

## Commit delegation and validation

For one through 32 transitions:

- call `CommitRuntimeStatus(ctx, reconciliation)` exactly once;
- pass the exact immutable reconciliation Candidate without rebuilding,
  truncating, reordering, or mutating it;
- do not retry on any result or error;
- propagate committer errors unchanged with both returned Candidates zero; and
- check context again after the committer returns.

A successful commit result is accepted only when all of these public immutable
facts match:

```text
Committed() == true
SourceReconciliationDigest() == reconciliation.CandidateDigest()
BaselineDigest() == reconciliation.BaselineDigest()
SourceDiscoveryDigest() == reconciliation.SourceDiscoveryDigest()
EventCount() == reconciliation.TransitionCount()
len(Events()) == reconciliation.TransitionCount()
CommitDigest() is lowercase 64-character SHA-256 text
```

Zero, uncommitted, wrong-source, wrong-count, invalid-digest, or otherwise
inconsistent successful results return zero Candidates plus
`ErrRuntimeStatusCommitResultMismatch`.

The coordinator does not reconstruct or validate concrete Event envelopes or
payloads. Accepted S2-W23 remains authority for Event metadata, canonical Event
construction, atomic append, exact appender-result validation, idempotent retry,
and commit digest construction.

## Context, errors, and mutation

- Context cancellation/deadline errors are returned canonically at every
  observed boundary.
- No source, baseline, Candidate, Event slice, or returned accessor data is
  retained or mutated by the coordinator.
- A caller mutation after return cannot change accepted Candidate facts because
  S2-W22 and S2-W23 own immutable copied accessors.
- On every non-nil error, both returned Candidates are zero.

## Acceptance criteria

1. Nil/typed-nil committer, nil context, and canceled/expired context fail
   before reconciliation or commit with zero Candidates.
2. A valid zero-transition reconciliation returns exactly once with no commit.
3. One and multiple transitions preserve accepted S2-W22 ordering and delegate
   the exact immutable reconciliation once.
4. Invalid baseline/discovery, identity drift, and reconciliation cancellation
   propagate unchanged with zero commit calls.
5. Committer sentinel and context errors propagate unchanged, with no retry and
   zero returned Candidates.
6. Every successful result fact listed above is checked; each mismatch returns
   `ErrRuntimeStatusCommitResultMismatch` and zero Candidates.
7. Input/Candidate/accessor/result mutation is isolated.
8. A real temporary SQLite integration uses accepted S2-W23 to append the exact
   status Event batch once and proves an explicit exact retry is stable.
9. Existing S2-W2, S2-W20 through S2-W28, Journal, state, projection, Runtime,
   and repository behavior remains green.
10. No discovery execution, discovery Event append, baseline construction,
    projection/Journal/SQLite direct access, metadata allocation, status policy
    invention, status-from-absence inference, scheduler/ticker/sleep/goroutine/
    daemon, process-policy change, Runtime selection/reservation/activation,
    Agent/model/Team/Work/Run/Grant/Bridge behavior, credential/network/user
    state access, external action, or Slice 3 behavior is introduced.

## Mandatory RED tests

The Developer first creates only `internal/app/runtime_status_test.go`. RED must
fail on the missing frozen errors, port, and coordinator symbols.

Required groups named
`TestRunObservedRuntimeStatusReconciliationOnce...`:

1. nil/typed-nil committer and nil/canceled/expired context prevalidation;
2. valid zero-transition no-commit behavior;
3. exact one/multiple transition reconciliation-to-commit delegation;
4. invalid baseline/discovery, identity drift, and context propagation;
5. committer error and every frozen successful-result mismatch;
6. baseline/source/Candidate/accessor/appender/result mutation isolation;
7. accepted S2-W23 plus real temporary SQLite exact append and explicit exact
   retry; and
8. static import, no-projection/no-direct-write/no-metadata-allocation/
   no-status-policy/no-scheduler/no-daemon/no-execution-authority/scope
   assertions.

Tests use deterministic S2-W2 fake probes, accepted immutable Candidates,
in-memory fakes, and temporary SQLite only. They do not invoke S2-W18, an
installed Pi executable, user Pi state, credentials, network, package manager,
daemon, real Runtime process, Agent/model call, or external action.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/app -run
  'TestRunObservedRuntimeStatusReconciliationOnce' -count=1`
- Package full: `go test ./internal/app -count=1`
- Impact:
  `go test ./internal/app ./internal/runtime ./internal/state
  ./internal/projection ./internal/journal -count=1`
- Repeated focused race:
  `go test -race ./internal/app -run
  'TestRunObservedRuntimeStatusReconciliationOnce' -count=50`
- Repository: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/app/runtime_status.go
  internal/app/runtime_status_test.go` and `git diff --check`
- Import boundary: product is standard library plus accepted
  `internal/runtime` and `internal/state`; static assertions reject projection,
  Journal/SQLite, concrete adapters, config, file/process/network, scheduler,
  daemon, CLI/UI, Team/Work/Run/Grant/Bridge, or Slice 3 imports.
- Scope: only the frozen product/test files, S2-W29 evidence,
  `docs/CURRENT.md`, and Controller-owned non-historical `PROGRESS.md` changes
  may enter the Candidate.

## Review and commit policy

- Fresh independent contract Reviewer `PASS` is required before the mandatory
  RED.
- Fresh independent implementation Reviewer `PASS` is required after the
  complete matrix.
- Up to three bounded contract-reviewed repairs are permitted.
- Only after all required checks and reviews pass may the Controller create one
  exact-scope local atomic commit.
- Push, merge, rebase, reset, release, credential/config mutation, dependency
  installation, external mutation, paid remote work, daemon/Runtime activation,
  and Slice 3 work remain prohibited.

## Explicit exclusions

No discovery execution or discovery Event write, baseline construction,
projection/Journal/SQLite direct access, Event metadata preparation/allocation,
status-from-absence inference, recurring scheduler/ticker/sleep/goroutine/
daemon/config entry, installed Runtime lookup, Pi process-policy change,
RuntimeProfile selection, capacity reservation, Runtime activation,
Team/Agent/WorkItem/Run/Evidence/grant/resource mutation, workspace, process,
model, Bridge, claim, lease, credential, network, filesystem, environment,
CLI/UI, external action, or Slice 3 behavior.

VERDICT: CONTRACT_FROZEN
