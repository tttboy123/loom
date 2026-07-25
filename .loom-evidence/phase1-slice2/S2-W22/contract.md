# S2-W22 Frozen WorkItem Contract

- ID: `S2-W22`
- Title: Observed Runtime Status Reconciliation Candidate
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W21 and local S2-W21 commit `366bc48`
- Corresponds to: `TECH-PLAN.md §6, §13.1, §14 Slice 2, §15.8`,
  ADR-0002, and ADR-0003
- Frozen branch/head: `codex/loom-platform-slice2` at `366bc48`

## Owned files

- `internal/runtime/status_reconciliation.go`
- `internal/runtime/status_reconciliation_test.go`
- `.loom-evidence/phase1-slice2/S2-W22/deliverable.md`

All accepted Slice 1 and S2-W1 through S2-W21 product/test files remain
unchanged. Any ownership expansion requires a recorded Controller amendment
and fresh contract Reviewer `PASS`.

## Objective

Create one pure immutable Candidate that compares:

1. a bounded caller-supplied baseline copied from accepted S2-W21 Runtime
   inventory; and
2. one accepted S2-W2 Runtime discovery snapshot.

The Candidate reports explicit status transitions only for RuntimeInstance IDs
present in both sources with stable device/adapter identity and different
accepted statuses.

This WorkItem does not append `RuntimeInstanceStatusChanged`, update a
projection, run discovery, infer status from absence, select a RuntimeProfile,
reserve capacity, start a process, schedule a scan, or activate a Runtime.

## Frozen input

```go
type RuntimeStatusBaseline struct {
    Instance              RuntimeInstance
    LastDiscoveryEventID  string
    LastDiscoverySequence int64
}
```

The caller owns the bounded status-baseline conversion from accepted S2-W21
read-model records: all `RuntimeInstance` fields plus the discovery Event ID
and sequence are copied; projection-only model/source/digest/time metadata is
not part of status comparison. The pure function treats this input as
untrusted and revalidates:

- zero through 32 baseline entries;
- each complete canonical RuntimeInstance through `NewRuntimeInstance` with
  canonical-form equality;
- nonempty last discovery Event ID and positive last discovery sequence; and
- unique RuntimeInstance IDs.

The current `RuntimeDiscoverySnapshot` may contain zero through 32
observations. Because its fields are package-private and nonzero snapshots are
created through accepted S2-W2 discovery, the reconciler revalidates the
snapshot digest, observation ordering, RuntimeInstances, source probe IDs,
model ordering, and unique IDs without inventing new catalog rules.

An empty current snapshot is a valid completed observation result but carries
no authority to mark missing baseline entries offline.

## Frozen transition and Candidate

```go
type RuntimeStatusTransition struct {
    RuntimeInstanceID     string
    DeviceID              string
    AdapterType           string
    FromStatus            RuntimeStatus
    ToStatus              RuntimeStatus
    SourceProbeID         string
    SourceDiscoveryDigest string
    PreviousEventID       string
    PreviousSequence      int64
}
```

The immutable Candidate has private fields and copied accessors:

```go
Reconciled() bool
BaselineDigest() string
SourceDiscoveryDigest() string
Transitions() []RuntimeStatusTransition
TransitionCount() int
CandidateDigest() string
```

The Candidate digest binds a version tag, the canonical complete baseline
digest, source discovery digest, ordered complete transitions, and transition
count. All digests are lowercase SHA-256.

Transitions are sorted by RuntimeInstance ID and are independent of baseline
input order. Every accessor returns copied data.

## Reconciliation semantics

- Matching ID plus equal status produces no transition.
- Matching ID plus different accepted status produces exactly one transition.
- Every accepted status pair is permitted when values differ: online,
  offline, incompatible, and disabled.
- Matching ID with changed DeviceID or AdapterType fails closed.
- A current observation absent from the baseline is a new discovery and
  produces no status transition; accepted S2-W20 owns its discovered Event.
- A baseline entry absent from the current snapshot produces no transition.
  Missing probe coverage is not represented by S2-W2, so absence cannot prove
  offline state.
- Display name, executable version, capabilities, capacity, models, or source
  probe changes do not themselves create a status transition. They remain
  Runtime discovery inventory facts.
- No source or Candidate input is mutated.

## Acceptance criteria

1. Same-status matching observations return a valid zero-transition Candidate.
2. Each distinct accepted status change returns one exact transition.
3. Multiple transitions are deterministically ordered by RuntimeInstance ID.
4. Baseline reordering yields identical baseline and Candidate digests.
5. Empty current discovery, baseline-only IDs, and current-only IDs return no
   fabricated status transitions.
6. Device or adapter identity drift fails closed with a zero Candidate.
7. Invalid/duplicate/oversized baseline, zero/invalid/oversized discovery,
   invalid source digest/order/model/probe/runtime content, nil/canceled/
   expired context, and mutation attempts fail closed or remain isolated as
   applicable.
8. Candidate digest changes for every transition field, source discovery
   digest, baseline semantic field, ordering-independent baseline set change,
   and transition count.
9. Existing Runtime catalog/discovery, S2-W20 writer, S2-W21 projection, and all
   accepted repository behavior remains green.
10. No Event append, Journal/projection/schema/file/config change, discovery
    execution, status-from-absence inference, RuntimeProfile selection,
    capacity reservation, Runtime activation, process/model/network/filesystem/
    environment/credential activity, scheduler/daemon/CLI/UI, external action,
    or Slice 3 behavior is introduced.

## Mandatory RED tests

The Developer first creates only the frozen test file. RED must fail on missing
S2-W22 baseline/transition/Candidate/reconciliation symbols.

Required groups named `TestReconcileObservedRuntimeStatuses...`:

1. zero and every accepted status transition;
2. deterministic multi-transition order and baseline reorder;
3. empty/current-only/baseline-only no-absence-inference behavior;
4. device/adapter drift and invalid baseline/source/context matrices;
5. source/input/Candidate/accessor mutation isolation;
6. complete digest sensitivity; and
7. static pure-domain import, no-write/no-execution, non-disclosure, and scope
   assertions.

Tests use only deterministic fake S2-W2 probes. They must not invoke
S2-W18/S2-W19, an installed Pi, user Pi state, network, credentials, package
managers, or a long-lived Runtime.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/runtime -run 'TestReconcileObservedRuntimeStatuses'
  -count=1`
- Package full: `go test ./internal/runtime -count=1`
- Impact:
  `go test ./internal/runtime ./internal/state ./internal/projection -count=1`
- Repeated focused race:
  `go test -race ./internal/runtime -run
  'TestReconcileObservedRuntimeStatuses' -count=50`
- Repository: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/runtime/status_reconciliation.go
  internal/runtime/status_reconciliation_test.go` and `git diff --check`
- Import boundary: product is standard library plus its own `internal/runtime`
  package. It must not import Journal, state, projection, runtime adapters, SQL,
  process, network, filesystem, config, credential, scheduler, daemon, UI, or
  Slice 3 packages.
- Scope: only the frozen product/test, S2-W22 evidence, `docs/CURRENT.md`, and
  Controller-owned non-historical `PROGRESS.md` changes may enter the
  Candidate.

## Explicit exclusions

No Event construction/append, Journal/projection/schema/file/config mutation,
discovery execution, probe invocation, status-from-absence inference,
`RuntimeInstanceStatusChanged` persistence/projection, RuntimeProfile
selection, capacity reservation, Runtime activation, Team/Agent/WorkItem/Run/
Evidence/grant/resource mutation, workspace, process, model, Bridge, claim,
lease, scheduler, daemon/CLI/UI, network, filesystem, environment, credential,
production goroutine, external action, or Slice 3 behavior.

VERDICT: CONTRACT_FROZEN
