# S2-W30 Frozen WorkItem Contract

- ID: `S2-W30`
- Title: Prepared Runtime Status Committer Adapter
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W29 local commit `51d0489`
- Corresponds to: `TECH-PLAN.md §6, §13.1, §14 Slice 2`,
  ADR-0002, ADR-0003, and accepted S2-W23/S2-W29
- Frozen branch/head: `codex/loom-platform-slice2` at `51d0489`

## Owned files

- `internal/app/runtime_status_committer.go`
- `internal/app/runtime_status_committer_test.go`
- `.loom-evidence/phase1-slice2/S2-W30/deliverable.md`

Slice 1 and S2-W1 through S2-W29 product/test files remain unchanged. Any
ownership expansion requires a recorded Controller amendment and fresh contract
Reviewer `PASS`.

## Objective

Add the smallest concrete adapter for accepted S2-W29
`RuntimeStatusCommitter`:

1. bind one accepted S2-W23 `state.EventBatchAppender`;
2. bind one injected provider of caller-authoritative S2-W23 commit input;
3. request input exactly once for the exact immutable reconciliation Candidate;
4. delegate exactly once to accepted
   `state.CommitRuntimeStatusTransitions`; and
5. return the exact S2-W23 immutable commit Candidate/error.

The adapter does not allocate Event IDs, idempotency keys, sequences,
reconciliation IDs, or time. It does not build projection baselines, run
discovery/reconciliation, infer status from absence, access concrete
Journal/SQLite/projection/config/files, retry, schedule, start a daemon, select
or reserve a Runtime, or activate a Runtime.

## Frozen public boundary

```go
var (
    ErrInvalidPreparedRuntimeStatusCommitter = errors.New(
        "invalid prepared runtime status committer",
    )
    ErrRuntimeStatusCommitInputFailed = errors.New(
        "runtime status commit input failed",
    )
)

type RuntimeStatusCommitInputProvider interface {
    PrepareRuntimeStatusCommit(
        context.Context,
        runtime.RuntimeStatusReconciliationCandidate,
    ) (state.RuntimeStatusCommitInput, error)
}

type PreparedRuntimeStatusCommitter struct {
    // private bindings
}

func NewPreparedRuntimeStatusCommitter(
    appender state.EventBatchAppender,
    provider RuntimeStatusCommitInputProvider,
) (*PreparedRuntimeStatusCommitter, error)

func (c *PreparedRuntimeStatusCommitter) CommitRuntimeStatus(
    ctx context.Context,
    candidate runtime.RuntimeStatusReconciliationCandidate,
) (state.RuntimeStatusCommitCandidate, error)
```

`*PreparedRuntimeStatusCommitter` statically satisfies accepted S2-W29
`RuntimeStatusCommitter`.

The product imports only standard library plus accepted `internal/runtime` and
`internal/state`. It does not import concrete Journal/SQLite, projection,
discovery/config, Runtime adapters, filesystem/process/network, scheduler,
daemon, CLI/UI, Team/Work/Run/Grant/Bridge, credentials, or Slice 3 packages.

## Construction and method-boundary validation

Construction rejects nil or typed-nil appender/provider with
`ErrInvalidPreparedRuntimeStatusCommitter` and invokes neither dependency.

Every method call revalidates, before context inspection or dependency use:

- nonnil receiver;
- nonnil/non-typed-nil stored appender;
- nonnil/non-typed-nil stored provider; and
- nonnil context.

Invalid method bindings return a zero Candidate plus
`ErrInvalidPreparedRuntimeStatusCommitter`. This explicitly includes the
exported zero value:

```go
var adapter PreparedRuntimeStatusCommitter
```

The zero value must fail closed without panic and without provider/appender
calls. A canceled or expired nonnil context is returned canonically before the
provider.

## Provider semantics

For a valid request:

- call `PrepareRuntimeStatusCommit(ctx, candidate)` exactly once;
- pass the exact immutable reconciliation Candidate;
- do not mutate or retain Candidate/accessor data;
- do not inspect environment, configuration, filesystem, projection, IDs,
  time, stream heads, credentials, or external state; and
- do not retry.

Canonical `context.Canceled` and `context.DeadlineExceeded` provider errors are
returned canonically. Any other provider error is wrapped so
`errors.Is(err, ErrRuntimeStatusCommitInputFailed)` and
`errors.Is(err, providerErr)` are both true. Every provider error returns a zero
Candidate and makes zero appender calls.

Context is checked again after provider return and before S2-W23 delegation.

## S2-W23 delegation

On valid provider output, call
`state.CommitRuntimeStatusTransitions(ctx, appender, candidate, input)` exactly
once with the original bound appender, exact immutable reconciliation
Candidate, and exact prepared input.

Accepted S2-W23 remains authority for:

- zero/invalid/oversized reconciliation Candidates;
- complete Candidate and digest revalidation;
- transition-to-metadata bijection;
- exact-next sequences and prior Event provenance;
- canonical `RuntimeInstanceStatusChanged` Event construction;
- payload and non-disclosure rules;
- atomic append and exact appender-result validation;
- exact retry/conflict behavior; and
- immutable commit Candidate construction.

The adapter does not duplicate, weaken, reinterpret, or retry S2-W23. S2-W23
errors propagate unchanged with its zero Candidate. A successful S2-W23
Candidate is returned exactly.

## Acceptance criteria

1. Constructor rejects nil/typed-nil bindings with no dependency calls.
2. Nil receiver, exported zero value, nil/typed-nil stored bindings, nil
   context, and canceled/expired context fail closed before provider/appender
   use.
3. The concrete adapter statically and dynamically satisfies S2-W29
   `RuntimeStatusCommitter`.
4. The provider receives the exact immutable reconciliation Candidate exactly
   once before any appender call.
5. Provider sentinel/context failures preserve frozen error identity, return a
   zero Candidate, make zero appender calls, and never retry.
6. Valid prepared input delegates exactly once to S2-W23 and returns its exact
   immutable Candidate/error.
7. S2-W23 zero/source/input/appender/result/context failures remain unchanged
   and are not retried.
8. Input, source, provider, appender batch/result, Event payload, Candidate, and
   accessor mutations remain isolated.
9. A real temporary SQLite integration wires accepted S2-W29 through this
   adapter to S2-W23 and proves exact append plus explicit exact retry.
10. Existing S2-W22 through S2-W29, Journal, state, projection, Runtime, and
    repository behavior remains green.
11. No metadata allocation, Event construction, concrete Journal/SQLite/
    projection/config/file access, discovery/reconciliation execution, status
    policy or absence inference, scheduler/ticker/sleep/goroutine/daemon,
    process policy change, Runtime selection/reservation/activation, Agent/
    model/Team/Work/Run/Grant/Bridge behavior, credential/network/user-state
    access, external action, or Slice 3 behavior is introduced.

## Mandatory RED tests

The Developer first creates only
`internal/app/runtime_status_committer_test.go`. RED must fail on the missing
frozen errors, provider, adapter, and constructor symbols.

Required groups named `TestPreparedRuntimeStatusCommitter...`:

1. constructor nil/typed-nil binding validation;
2. nil receiver, exported zero value, stored typed-nil bindings, and nil/
   canceled/expired context prevalidation;
3. static and dynamic S2-W29 interface conformance;
4. exact provider-before-appender ordering, once counts, exact Candidate/input,
   and source/Candidate/accessor mutation isolation;
5. provider sentinel, wrapped context, cancellation, and deadline errors;
6. S2-W23 zero/invalid source/input, appender failure/result mismatch/context
   propagation and zero Candidate behavior;
7. accepted S2-W29 plus real temporary SQLite first append and explicit exact
   retry; and
8. static import, no-metadata-allocation/no-Event-construction/no-projection/
   no-discovery/no-status-policy/no-scheduler/no-daemon/no-execution-authority/
   scope assertions.

Tests use deterministic accepted S2-W22 Candidates, in-memory fakes, and
temporary SQLite only. They do not invoke S2-W18 or installed Pi, inspect user
Pi state, access credentials/network, start a daemon, or activate a Runtime.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/app -run 'TestPreparedRuntimeStatusCommitter' -count=1`
- Package full: `go test ./internal/app -count=1`
- Impact:
  `go test ./internal/app ./internal/runtime ./internal/state
  ./internal/projection ./internal/journal -count=1`
- Repeated focused race:
  `go test -race ./internal/app -run
  'TestPreparedRuntimeStatusCommitter' -count=50`
- Repository: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/app/runtime_status_committer.go
  internal/app/runtime_status_committer_test.go` and `git diff --check`
- Import boundary: product is standard library plus accepted
  `internal/runtime` and `internal/state`; static assertions reject concrete
  Journal/SQLite/projection/adapters/config/file/process/network, scheduler,
  daemon, CLI/UI, Team/Work/Run/Grant/Bridge, credential, or Slice 3 imports.
- Scope: only the frozen product/test files, S2-W30 evidence,
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

No Event metadata preparation/allocation, Event construction, concrete Journal/
SQLite/projection/config/file access, stream-head read, baseline construction,
discovery/reconciliation invocation, status-from-absence inference, recurring
scheduler/ticker/sleep/goroutine/daemon/config entry, installed Runtime lookup,
Pi process-policy change, RuntimeProfile selection, capacity reservation,
Runtime activation, Team/Agent/WorkItem/Run/Evidence/grant/resource mutation,
workspace, process, model, Bridge, claim, lease, credential, network,
filesystem, environment, CLI/UI, external action, or Slice 3 behavior.

VERDICT: CONTRACT_FROZEN
