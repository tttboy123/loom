# S2-W27 Frozen WorkItem Contract

- ID: `S2-W27`
- Title: One-Shot Configured Runtime Discovery Commit Coordination
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W26 local commit `494579d`
- Corresponds to: `TECH-PLAN.md §6, §13.1, §14 Slice 2`,
  ADR-0002, ADR-0003, and accepted S2-W20/S2-W26
- Frozen branch/head: `codex/loom-platform-slice2` at `494579d`

## Owned files

- `internal/app/runtime_discovery.go`
- `internal/app/runtime_discovery_test.go`
- `.loom-evidence/phase1-slice2/S2-W27/deliverable.md`

The Developer owns only these files. Controller-owned contract, review, status,
and documentation evidence remain outside Developer ownership. All accepted
Slice 1 and S2-W1 through S2-W26 product/test files remain unchanged. Any
ownership expansion requires a recorded Controller amendment and fresh
contract Reviewer `PASS`.

## Objective

Add the smallest caller-triggered application coordination boundary that:

1. validates one injected Runtime discovery committer before observation;
2. runs accepted S2-W26 configured discovery exactly once;
3. treats an accepted empty/all-absent snapshot as a successful no-commit scan;
4. passes one non-empty accepted snapshot exactly once to the injected
   committer;
5. accepts only an exact S2-W20 immutable commit Candidate bound to that
   snapshot; and
6. returns no usable partial result on any failure.

This is one explicit command invocation, not a scheduler. The caller chooses to
invoke the discovery-commit path. The boundary does not choose between
rediscovery and status reconciliation, invoke S2-W22/S2-W23, infer status from
absence, allocate Event metadata, read projection state, update a read model,
start a daemon, or activate a Runtime.

## Frozen package and API

New package: `internal/app`.

```go
var (
    ErrInvalidRuntimeDiscoveryRun = errors.New(
        "invalid runtime discovery run",
    )
    ErrRuntimeDiscoveryCommitResultMismatch = errors.New(
        "runtime discovery commit result mismatch",
    )
)

type RuntimeDiscoveryCommitter interface {
    CommitRuntimeDiscovery(
        context.Context,
        runtime.RuntimeDiscoverySnapshot,
    ) (state.RuntimeDiscoveryCommitCandidate, error)
}

func RunConfiguredRuntimeDiscoveryOnce(
    ctx context.Context,
    factories []discoveryscan.ProbeFactory,
    committer RuntimeDiscoveryCommitter,
) (
    runtime.RuntimeDiscoverySnapshot,
    state.RuntimeDiscoveryCommitCandidate,
    error,
)
```

The narrow committer port binds Event metadata and the accepted S2-W20
`EventBatchAppender` outside this coordinator. Product code does not allocate
IDs, timestamps, idempotency keys, stream sequences, or correlation IDs.

## Input prevalidation

- nil context returns two zero values plus
  `ErrInvalidRuntimeDiscoveryRun`;
- canceled/deadline context returns the exact context error before any factory
  or committer call;
- nil or typed-nil committer returns two zero values plus
  `ErrInvalidRuntimeDiscoveryRun` before any factory call;
- the factory slice and every factory/result remain governed by accepted
  S2-W26, including its zero-through-32 bound, copy, prevalidation, caller
  order, typed-nil, canonical absence, and no-observation-before-collection
  rules; and
- the coordinator checks context immediately before and after the committer
  call.

The coordinator does not duplicate or weaken S2-W26 validation.

## Empty observation

An accepted S2-W26 snapshot with:

```text
Digest() != ""
len(Observations()) == 0
```

returns that exact valid immutable snapshot, a zero
`RuntimeDiscoveryCommitCandidate`, and nil error. The committer is not called.
This is a completed observation with no discovered facts to record, matching
accepted S2-W20's explicit refusal to record empty snapshots.

Empty/all-absent never deletes a projected Runtime or fabricates offline,
incompatible, disabled, removed, or status-changed facts.

## Non-empty commit

For one non-empty accepted snapshot:

1. check context;
2. call `CommitRuntimeDiscovery(ctx, snapshot)` exactly once;
3. propagate a committer/context error and return two zero values;
4. check context again; and
5. accept the result only when all are true:

```text
candidate.Committed() == true
candidate.SourceDiscoveryDigest() == snapshot.Digest()
candidate.EventCount() == len(snapshot.Observations())
len(candidate.Events()) == candidate.EventCount()
candidate.CommitDigest() is lowercase SHA-256
```

`RuntimeDiscoveryCommitCandidate` has private state and is minted by accepted
S2-W20. A zero, wrong-source, wrong-count, or otherwise inconsistent successful
result returns two zero values plus
`ErrRuntimeDiscoveryCommitResultMismatch`.

The committer receives an immutable snapshot value whose accessors return
copies. The coordinator returns the original accepted immutable snapshot and
the exact accepted immutable commit Candidate. Caller/factory/probe/committer
input, committer accessors, and returned accessors cannot mutate each other or
cause a repeated call.

## Error and no-partial semantics

- S2-W26 errors propagate unchanged and the committer is not called.
- Committer errors remain inspectable and return two zero values.
- Context cancellation/deadline remains inspectable and returns two zero
  values.
- A nil-error invalid commit Candidate returns
  `ErrRuntimeDiscoveryCommitResultMismatch` and two zero values.
- Product error text must not add factory values, Runtime observations, Event
  payloads, IDs, paths, environment, output, credentials, prompts, sessions,
  or raw Candidate contents.

The accepted S2-W20 appender is atomic, but a caller-supplied committer remains
responsible for its own side-effect contract. This coordinator never reports a
failed or mismatched commit as successful and never retries it.

## Acceptance criteria

1. Invalid context/committer fails before every factory call.
2. Empty and all-absent scans return the exact valid empty snapshot, zero
   commit Candidate, and no committer call.
3. One or multiple present factories retain S2-W26 build order and S2-W2 probe
   order, then call the committer once with the exact immutable snapshot.
4. An exact accepted S2-W20 Candidate with matching source digest and event
   count is returned unchanged.
5. Factory/probe/discovery/context failures prevent commit and return no
   partial result.
6. Committer error, context cancellation, zero/mismatched Candidate, or
   accessor/count/digest mismatch returns no partial result and is never
   retried.
7. Caller/factory/probe/snapshot/committer/Candidate/accessor mutation cannot
   change accepted output or call counts.
8. A deterministic real SQLite integration binds a test committer to accepted
   S2-W20, proves one non-empty S2-W26 scan appends the exact canonical batch
   once, and proves exact retry remains idempotent.
9. Existing S2-W2, S2-W14, S2-W19 through S2-W26, Journal, state, projection,
   and repository behavior remains green.
10. No Event metadata allocation, direct Journal/SQLite call, projection
    query/update, status reconciliation/Event, status-from-absence inference,
    scheduler/ticker/sleep/goroutine/daemon, process policy change, Runtime
    selection/reservation/activation, Team/Agent/WorkItem/Run/Grant/Evidence
    mutation, model call, network, credential, external action, or Slice 3
    behavior is added.

## Mandatory RED tests

Before product code, add only `internal/app/runtime_discovery_test.go`. The
focused RED command must fail only because the frozen package symbols do not
exist.

Required groups named `TestRunConfiguredRuntimeDiscoveryOnce...`:

1. nil/typed-nil committer and nil/canceled/deadline context prevalidation with
   zero factory calls;
2. empty/all-absent valid snapshot and zero committer calls;
3. exact factory/probe/commit order, one commit call, exact snapshot/Candidate,
   and mutation isolation;
4. factory/probe/discovery/context failure with zero commit calls and zero
   outputs;
5. committer source error, cancellation, zero/mismatched Candidate, no retry,
   error inspectability, and non-disclosure;
6. accepted S2-W20 plus real temporary SQLite Journal exact append/retry
   integration; and
7. static imports plus no-ID-allocation/no-direct-Journal/no-projection/
   no-status/no-scheduler/no-daemon/no-execution-authority/scope assertions.

Tests use deterministic fake factories/probes and temporary SQLite state only.
They do not invoke S2-W18 or an installed Pi executable, inspect user Pi state,
access credentials/network, start a daemon, or activate a Runtime.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/app -run
  'TestRunConfiguredRuntimeDiscoveryOnce' -count=1`
- Package:
  `go test ./internal/app -count=1`
- Impact:
  `go test ./internal/app ./internal/runtime
  ./internal/runtime/discoveryscan ./internal/state ./internal/journal
  -count=1`
- Repeated focused race:
  `go test -race ./internal/app -run
  'TestRunConfiguredRuntimeDiscoveryOnce' -count=50`
- Repository: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/app/runtime_discovery.go
  internal/app/runtime_discovery_test.go` and `git diff --check`
- Product imports: standard library plus accepted `internal/runtime`,
  `internal/runtime/discoveryscan`, and `internal/state` only. Product must not
  import the concrete Pi adapter, Journal/SQLite/projection/config/credential,
  process/network/filesystem, scheduler/daemon/CLI/UI, Team/Work/Run/Grant/
  Evidence, or Slice 3 packages.
- Scope: only the two frozen product/test files, S2-W27 evidence,
  `docs/CURRENT.md`, and Controller-owned non-historical `PROGRESS.md` changes
  may enter the Candidate.

## Explicit exclusions

No ID/time/sequence/idempotency/correlation allocation, concrete appender,
Journal/SQLite/projection/config/file access, status baseline/reconciliation/
Event, absence inference, scan scheduling/retry/ticker/sleep/goroutine/daemon,
Pi lookup/process policy change, installed Pi/user Pi state, RuntimeProfile
selection, capacity reservation, Runtime activation, Team/Draft/Agent/
WorkItem/Run/Evidence/Grant/resource mutation, workspace, Bridge, claim, lease,
model/Agent call, network, credential, external action, push/merge/rebase/
reset/release, or Slice 3 behavior.

VERDICT: CONTRACT_FROZEN
