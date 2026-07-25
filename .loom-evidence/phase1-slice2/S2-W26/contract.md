# S2-W26 Frozen WorkItem Contract

- ID: `S2-W26`
- Title: Bounded Configured Runtime Discovery Scan
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W25 local commit `38d914b`
- Corresponds to: `TECH-PLAN.md §4, §13.1, §14 Slice 2`,
  accepted S2-W2/S2-W17/S2-W18/S2-W19, ADR-0002, and ADR-0003
- Frozen branch/head: `codex/loom-platform-slice2` at `38d914b`

## Owned files

- `internal/runtime/discoveryscan/scan.go`
- `internal/runtime/discoveryscan/scan_test.go`
- `.loom-evidence/phase1-slice2/S2-W26/deliverable.md`

The Developer owns only these files. Controller-owned contract, review, status,
and documentation evidence remain outside Developer ownership. All accepted
Slice 1 and S2-W1 through S2-W25 product/test files remain unchanged. Any
ownership expansion requires a recorded Controller amendment and fresh contract
Reviewer `PASS`.

## Objective

Add the smallest one-shot coordination boundary between configured Runtime probe
factories and accepted S2-W2 discovery:

1. prevalidate a bounded caller-supplied factory set;
2. build each factory exactly once in caller order;
3. skip only an explicit canonical absent result;
4. collect each explicit present probe without invoking it during collection;
5. delegate complete probe validation, deterministic probe-ID order,
   observation, normalization, uniqueness, and snapshot digest to accepted
   `runtime.DiscoverRuntime`; and
6. return only the accepted immutable discovery snapshot or a zero snapshot on
   failure.

The boundary performs one caller-triggered scan. It does not schedule, repeat,
sleep, start a goroutine/daemon, allocate Event metadata, query or write the
Journal, build a baseline, reconcile status, append Events, update projection,
infer status from absence, select/reserve/activate a Runtime, or invoke an Agent
or model.

## Frozen package and API

New package: `internal/runtime/discoveryscan`.

```go
const MaxProbeFactories = 32

var (
    ErrInvalidConfiguredRuntimeDiscovery = errors.New(
        "invalid configured runtime discovery",
    )
    ErrRuntimeProbeFactoryFailed = errors.New(
        "runtime probe factory failed",
    )
)

type ProbeFactory interface {
    BuildProbe(context.Context) (runtime.RuntimeProbe, bool, error)
}

func DiscoverConfiguredRuntimes(
    ctx context.Context,
    factories []ProbeFactory,
) (runtime.RuntimeDiscoverySnapshot, error)
```

The interface exactly matches accepted S2-W19
`(*piadapter.PiLocalRuntimeProbeFactory).BuildProbe`; product code does not
import the concrete adapter.

## Input and factory validation

- nil context rejects with `ErrInvalidConfiguredRuntimeDiscovery`;
- canceled/deadline context returns the exact context error before any factory
  call;
- zero factories is valid and delegates an empty probe set to accepted S2-W2;
- one through 32 factories are accepted;
- more than 32 factories rejects without truncation or factory calls;
- nil and typed-nil factories reject before any factory call;
- the factory slice is copied before use;
- all factories are prevalidated before the first `BuildProbe` call.

Factories are called exactly once in caller order. Before and after each call,
context cancellation/deadline is checked. A context error from a factory remains
inspectable and returns the zero snapshot.

## Canonical factory result

The only accepted result shapes are:

```text
absent:  probe == nil, present == false, err == nil
present: probe != nil and not typed-nil, present == true, err == nil
```

These fail with `ErrInvalidConfiguredRuntimeDiscovery` and a zero snapshot:

- nil or typed-nil probe with `present=true`;
- nonnil probe with `present=false`; and
- any other contradictory result shape.

A non-context factory error returns a zero snapshot and an error matching both
`ErrRuntimeProbeFactoryFailed` and the original source error. Error text must
not add factory values, paths, environment, output, credentials, or probe data.

Explicit absence is not an error and adds no probe. An all-absent scan delegates
an empty probe set to S2-W2 and returns the same deterministic valid empty
snapshot as direct accepted discovery. Absence never fabricates an offline,
incompatible, disabled, removed, or status-changed Runtime.

## Delegation to accepted discovery

After every factory succeeds, the collected probe slice is passed exactly once
to `runtime.DiscoverRuntime`.

Accepted S2-W2 remains authority for:

- nil/typed-nil and duplicate probes;
- nonempty captured probe IDs;
- deterministic probe invocation order by captured probe ID;
- exactly-once `ObserveRuntime` invocation;
- probe/context failures;
- RuntimeInstance, SourceProbeID, model, capability, and duplicate-instance
  validation;
- canonical observation order and copied immutable data; and
- deterministic lowercase SHA-256 snapshot digest.

This WorkItem does not duplicate or weaken any of those validators. Discovery
errors propagate unchanged, and every failure returns the zero snapshot.

## Acceptance criteria

1. Zero factories and all-absent factories return equal valid nonzero-digest
   empty snapshots with zero observations.
2. Mixed absent/present factories are built once in caller order; only present
   probes are observed once through accepted S2-W2 in deterministic probe-ID
   order.
3. The exact accepted snapshot, observation order, SourceProbeID, Runtime
   content, model inventory, copying behavior, and digest are preserved.
4. Invalid count, nil/typed-nil factory, nil/typed-nil present probe,
   contradictory result shape, factory error, invalid/duplicate probe,
   observation failure/content error, and context failure return a zero
   snapshot.
5. All factories are prevalidated before calls; a failure at factory `N`
   prevents later factory calls and prevents every probe observation.
6. Input-slice/factory-result/probe-observation/snapshot accessor mutation
   cannot alter the returned snapshot or cause repeated calls.
7. `*piadapter.PiLocalRuntimeProbeFactory` statically satisfies `ProbeFactory`;
   an empty deterministic temporary search configuration returns explicit
   absence without starting a process.
8. Existing S2-W2 and S2-W17 through S2-W25 behavior remains green.
9. No Event/Journal/state/projection/schema/config mutation, baseline/status
   reconciliation, status-from-absence inference, scheduler/ticker/sleep/
   goroutine/daemon, long-lived process, installed Pi, user Pi state,
   credential, network, package manager, RuntimeProfile selection, capacity
   reservation, Runtime activation, Team/Agent/WorkItem/Run/Grant/Evidence
   mutation, model/Agent call, external action, or Slice 3 behavior is added.

## Mandatory RED tests

Before product code, add only
`internal/runtime/discoveryscan/scan_test.go`. The focused RED command must fail
only because the frozen package symbols do not exist.

Required groups named `TestDiscoverConfiguredRuntimes...`:

1. empty/all-absent equality and no status inference;
2. exact factory call order, present filtering, accepted probe-ID observation
   order, snapshot content, digest, and accessor isolation;
3. count/nil/typed-nil prevalidation with zero calls;
4. all contradictory factory result shapes and factory/context failures with
   zero snapshot, no later calls, and no observations;
5. accepted S2-W2 invalid/duplicate probe, observation, content, cancellation,
   and mutation behavior through the new boundary;
6. source-error inspectability and error non-disclosure; and
7. concrete S2-W19 interface/absent integration plus static import/no-write/
   no-persistence/no-scheduler/no-execution-authority/scope assertions.

Tests use deterministic fake factories/probes and one temporary empty-directory
S2-W19 factory. They do not invoke an installed Pi executable, start S2-W18,
read user Pi state, access credentials/network, or start a daemon.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/runtime/discoveryscan -run
  'TestDiscoverConfiguredRuntimes' -count=1`
- Package full:
  `go test ./internal/runtime/discoveryscan ./internal/runtime -count=1`
- S2-W17/S2-W18/S2-W19 impact:
  `go test ./internal/runtime ./internal/runtime/piadapter
  ./internal/runtime/discoveryscan -count=1`
- Repeated focused race:
  `go test -race ./internal/runtime/discoveryscan -run
  'TestDiscoverConfiguredRuntimes' -count=50`
- Repository: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/runtime/discoveryscan/scan.go
  internal/runtime/discoveryscan/scan_test.go` and `git diff --check`
- Import boundary: product code may import only standard library plus accepted
  `internal/runtime`. It must not import concrete Pi adapters, Journal/state/
  projection, SQL, filesystem/process/network/config/credential, scheduler,
  daemon, CLI/UI, Team/Work/Run/Grant/Evidence, or Slice 3 packages.
- Scope: only the two frozen product/test files, S2-W26 evidence,
  `docs/CURRENT.md`, and Controller-owned non-historical `PROGRESS.md` changes
  may enter the Candidate.

## Explicit exclusions

No Event metadata planning/allocation, Journal/state/projection read/write,
discovery persistence, baseline construction, status reconciliation/Event,
absence inference, recurring scheduler/ticker/sleep/goroutine/daemon/config
entrypoint, concrete adapter import, executable lookup policy change, isolation
policy change, installed Pi/user Pi state, long-lived Runtime process,
RuntimeProfile selection, capacity reservation, Runtime activation,
Team/Draft/Agent/WorkItem/Run/Evidence/Grant/resource mutation, workspace,
Bridge, claim, lease, model/Agent call, network, credential, external action,
push/merge/rebase/reset/release, or Slice 3 behavior.

VERDICT: CONTRACT_FROZEN
