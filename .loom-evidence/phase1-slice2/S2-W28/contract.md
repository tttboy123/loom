# S2-W28 Frozen WorkItem Contract

- ID: `S2-W28`
- Title: Prepared Runtime Discovery Committer Adapter
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W27 local commit `7ec635b`
- Corresponds to: `TECH-PLAN.md §6, §13.1, §14 Slice 2`,
  ADR-0002, ADR-0003, and accepted S2-W20/S2-W27
- Frozen branch/head: `codex/loom-platform-slice2` at `7ec635b`

## Owned files

- `internal/app/runtime_discovery_committer.go`
- `internal/app/runtime_discovery_committer_test.go`
- `.loom-evidence/phase1-slice2/S2-W28/deliverable.md`

The Developer owns only these files. Controller-owned contract, review, status,
and documentation evidence remain outside Developer ownership. All accepted
Slice 1 and S2-W1 through S2-W27 product/test files remain unchanged. Any
ownership expansion requires a recorded Controller amendment and fresh
contract Reviewer `PASS`.

## Objective

Add the smallest concrete adapter for accepted S2-W27
`RuntimeDiscoveryCommitter`:

1. bind one accepted S2-W20 `state.EventBatchAppender`;
2. bind one injected provider of caller-authoritative S2-W20 commit input;
3. prepare input exactly once for each explicit non-empty commit request;
4. delegate Event validation/construction/atomic append exactly once to
   accepted `state.CommitRuntimeDiscoverySnapshot`; and
5. return only its exact immutable Candidate or a zero Candidate on failure.

The adapter does not allocate Event IDs, timestamps, correlation/idempotency
IDs, or stream sequences. It does not discover Runtime instances, inspect
projection state, choose status-versus-rediscovery policy, retry, schedule,
start a daemon, or activate a Runtime.

## Frozen API

Same accepted package: `internal/app`.

```go
var (
    ErrInvalidPreparedRuntimeDiscoveryCommitter = errors.New(
        "invalid prepared runtime discovery committer",
    )
    ErrRuntimeDiscoveryCommitInputFailed = errors.New(
        "runtime discovery commit input failed",
    )
)

type RuntimeDiscoveryCommitInputProvider interface {
    PrepareRuntimeDiscoveryCommit(
        context.Context,
        runtime.RuntimeDiscoverySnapshot,
    ) (state.RuntimeDiscoveryCommitInput, error)
}

type PreparedRuntimeDiscoveryCommitter struct {
    // private immutable bindings
}

func NewPreparedRuntimeDiscoveryCommitter(
    appender state.EventBatchAppender,
    provider RuntimeDiscoveryCommitInputProvider,
) (*PreparedRuntimeDiscoveryCommitter, error)

func (c *PreparedRuntimeDiscoveryCommitter) CommitRuntimeDiscovery(
    ctx context.Context,
    snapshot runtime.RuntimeDiscoverySnapshot,
) (state.RuntimeDiscoveryCommitCandidate, error)
```

Static conformance is required:

```go
var _ RuntimeDiscoveryCommitter =
    (*PreparedRuntimeDiscoveryCommitter)(nil)
```

Product code imports no concrete Journal/SQLite implementation.

## Constructor validation

- nil or typed-nil appender rejects with
  `ErrInvalidPreparedRuntimeDiscoveryCommitter`;
- nil or typed-nil input provider rejects with the same error;
- invalid construction returns nil, never a partial adapter;
- accepted interfaces are bound exactly as provided and are not invoked by the
  constructor; and
- no environment, configuration, filesystem, projection, ID, time, or stream
  state is read.

The adapter uses the accepted S2-W20 `state.EventBatchAppender` port. It does
not duplicate appender behavior or import `internal/journal`.

## Commit request

`CommitRuntimeDiscovery`:

1. rejects nil receiver or nil context with a zero Candidate and
   `ErrInvalidPreparedRuntimeDiscoveryCommitter`;
2. returns exact canceled/deadline context before provider or appender use;
3. calls `PrepareRuntimeDiscoveryCommit(ctx, snapshot)` exactly once;
4. treats an exact or wrapped context error from the provider as the canonical
   exact context error;
5. wraps a non-context provider error so both
   `ErrRuntimeDiscoveryCommitInputFailed` and the source error remain
   inspectable;
6. checks context after provider return;
7. calls accepted `state.CommitRuntimeDiscoverySnapshot` exactly once with the
   original snapshot, bound appender, and prepared input; and
8. returns the exact accepted Candidate/error without reinterpretation or
   retry.

Accepted S2-W20 remains authority for zero/empty/invalid snapshots, complete
input coverage, Event IDs/keys/sequences, canonical Event/payload construction,
atomic append, exact result validation, Candidate digest, and all mutation
isolation.

## Error and no-partial semantics

- Invalid adapter/request returns a zero Candidate.
- Provider failure/cancellation returns a zero Candidate and never calls the
  appender.
- S2-W20 input/source/appender/context/result errors propagate unchanged and
  return the zero Candidate guaranteed by S2-W20.
- The adapter does not retry provider or S2-W20.
- Product error text does not add commit input, Event/payload/Candidate
  contents, IDs, Runtime observations, paths, environment, output, credentials,
  prompts, sessions, or bound object values.

Caller/provider input slices and snapshot/Candidate/Event accessors remain
isolated through accepted S2-W20/S2-W2 copying. The adapter stores no mutable
request or result state.

## Acceptance criteria

1. Constructor rejects every nil/typed-nil binding without invoking either.
2. The concrete adapter statically and dynamically satisfies S2-W27
   `RuntimeDiscoveryCommitter`.
3. Nil receiver/context and pre-canceled/deadline context produce zero
   Candidate and zero provider/appender calls.
4. Provider is called exactly once before the appender and receives the exact
   immutable snapshot.
5. Provider source/context failure is inspectable, returns zero Candidate,
   prevents append, and is not retried.
6. Valid prepared input delegates once to S2-W20 and returns its exact
   Candidate.
7. Invalid snapshot/input, appender failure/result mismatch, and cancellation
   preserve exact S2-W20 errors and zero Candidate.
8. Input/snapshot/appender/Candidate/accessor mutation cannot change accepted
   results or call counts.
9. A real temporary SQLite integration wires S2-W27 configured discovery
   through this adapter to accepted S2-W20 and proves exact append plus explicit
   idempotent retry.
10. Existing S2-W2, S2-W14, S2-W20, S2-W26, S2-W27, Journal, state,
    projection, and repository behavior remains green.
11. No ID/time/sequence/key allocation, concrete Journal/SQLite/projection/
    config/file access, status reconciliation/Event or policy, absence
    inference, scheduler/ticker/sleep/goroutine/daemon, process policy change,
    Runtime selection/reservation/activation, Team/Agent/WorkItem/Run/Grant/
    Evidence mutation, model call, network, credential, external action, or
    Slice 3 behavior is added.

## Mandatory RED tests

Before product code, add only
`internal/app/runtime_discovery_committer_test.go`. The focused RED command must
fail only because the frozen adapter/provider/error symbols do not exist.

Required groups named `TestPreparedRuntimeDiscoveryCommitter...`:

1. nil/typed-nil constructor bindings and no constructor calls;
2. static/dynamic S2-W27 interface conformance;
3. nil receiver/context and canceled/deadline request prevalidation;
4. exact provider-before-appender call order, once counts, exact snapshot/input,
   and exact accepted Candidate;
5. provider source/wrapped-context failure, cancellation after provider, zero
   append, no retry, inspectability, and non-disclosure;
6. S2-W20 empty/invalid source/input, appender failure/result mismatch/context
   propagation with zero Candidate;
7. mutation isolation; and
8. S2-W27 plus real temporary SQLite first append and explicit exact retry,
   with static no-allocation/no-concrete-Journal/no-projection/no-status/
   no-scheduler/no-daemon/no-execution-authority/scope assertions.

Tests use deterministic fake providers/appenders and temporary SQLite state.
They do not invoke S2-W18 or installed Pi, inspect user Pi state, access
credentials/network, start a daemon, or activate a Runtime.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/app -run
  'TestPreparedRuntimeDiscoveryCommitter' -count=1`
- Package: `go test ./internal/app -count=1`
- Impact:
  `go test ./internal/app ./internal/runtime
  ./internal/runtime/discoveryscan ./internal/state ./internal/journal
  -count=1`
- Repeated focused race:
  `go test -race ./internal/app -run
  'TestPreparedRuntimeDiscoveryCommitter' -count=50`
- Repository: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/app/runtime_discovery_committer.go
  internal/app/runtime_discovery_committer_test.go` and `git diff --check`
- Product imports: standard library plus accepted `internal/runtime` and
  `internal/state` only. Product must not import discoveryscan, concrete Pi
  adapter, Journal/SQLite/projection/config/credential, process/network/
  filesystem, scheduler/daemon/CLI/UI, Team/Work/Run/Grant/Evidence, or Slice 3
  packages.
- Scope: only the two frozen product/test files, S2-W28 evidence,
  `docs/CURRENT.md`, and Controller-owned non-historical `PROGRESS.md` changes
  may enter the Candidate.

## Explicit exclusions

No ID/time/sequence/idempotency/correlation allocation, discovery/probe call,
concrete Journal/SQLite/projection/config/file access, status baseline/
reconciliation/Event/policy, absence inference, scheduling/retry/ticker/sleep/
goroutine/daemon, Pi lookup/process policy change, installed Pi/user Pi state,
RuntimeProfile selection, capacity reservation, Runtime activation,
Team/Draft/Agent/WorkItem/Run/Evidence/Grant/resource mutation, workspace,
Bridge, claim, lease, model/Agent call, network, credential, external action,
push/merge/rebase/reset/release, or Slice 3 behavior.

VERDICT: CONTRACT_FROZEN
