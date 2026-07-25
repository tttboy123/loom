# S2-W25 Frozen WorkItem Contract

- ID: `S2-W25`
- Title: Runtime Status Baseline Projection Adapter
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W24 local commit `9838779`
- Corresponds to: `TECH-PLAN.md §6, §13.1, §14 Slice 2, §15.8`,
  ADR-0002, ADR-0003, accepted S2-W22/S2-W23/S2-W24, and the frozen S2-W22
  amendment in this evidence directory
- Frozen branch/head: `codex/loom-platform-slice2` at `9838779`

## Owned files

- `internal/runtime/status_reconciliation.go`
- `internal/runtime/status_reconciliation_test.go`
- `internal/state/runtime_status_writer_test.go`
- `internal/projection/runtime_status_test.go`
- `internal/projection/runtime_status_baseline.go`
- `internal/projection/runtime_status_baseline_test.go`
- `.loom-evidence/phase1-slice2/S2-W25/deliverable.md`

The Developer owns only these files. Controller-owned contract, amendment,
review, status, and documentation evidence remain outside Developer ownership.
All other accepted Slice 1 and S2-W1 through S2-W24 product/test files remain
unchanged. Any ownership expansion requires a recorded Controller amendment and
fresh contract Reviewer `PASS`.

The first four owned files are accepted files. Product changes in
`status_reconciliation.go` are limited to the frozen provenance-field rename,
baseline digest record rename, and baseline digest version increment. Changes
in the three accepted test files are limited to the rename plus direct
regression proof required by this contract. The projection adapter is a new
pure file and does not alter accepted Event replay or projection mutation.

## Objective

Build one bounded pure adapter from a copied S2-W24 `projection.Snapshot` to the
renamed S2-W22 `[]runtime.RuntimeStatusBaseline`:

1. treat `Snapshot.RuntimeInstances` as untrusted copied read-model input;
2. revalidate every consumed Runtime domain, discovery, inventory, and optional
   status-provenance field;
3. deterministically order output by RuntimeInstance ID;
4. select the latest canonical status-bearing Event ID/sequence; and
5. return copied canonical Runtime-domain values suitable for the next pure
   S2-W22 reconciliation.

The adapter does not mutate the Projection, query/replay the Journal, invoke
discovery/reconciliation, append Events, create a scheduler cycle, infer status
from absence, run Pi, select/reserve/activate a Runtime, or invoke any
process/model/Agent.

## Frozen S2-W22 amendment

The sibling `s2-w22-amendment.md` is part of this frozen contract. The accepted
baseline type becomes:

```go
type RuntimeStatusBaseline struct {
    Instance         RuntimeInstance
    PreviousEventID  string
    PreviousSequence int64
}
```

S2-W22 validation, cloning, reconciliation, and tests use the renamed fields.
The canonical baseline digest record uses `previous_event_id` and
`previous_sequence`; `runtimeStatusBaselineDigestVersion` becomes `2`.
The transition shape and `runtimeStatusCandidateDigestVersion` remain
unchanged.

## Frozen adapter API

```go
var ErrInvalidRuntimeStatusBaselineProjection = errors.New(
    "invalid runtime status baseline projection",
)

func BuildRuntimeStatusBaselines(
    snapshot Snapshot,
) ([]loomruntime.RuntimeStatusBaseline, error)
```

The function is deterministic and side-effect free. On any invalid input it
returns `nil` and an error matching
`ErrInvalidRuntimeStatusBaselineProjection`. It never returns a partial or
truncated baseline.

## Bounded source and canonical Runtime conversion

Only `snapshot.RuntimeInstances` is consumed. Other Snapshot maps do not grant
authority and are ignored.

- zero through 32 Runtime map entries are accepted;
- more than 32 entries are rejected rather than truncated;
- a nil or empty Runtime map returns a non-nil empty baseline;
- every map key must equal the nonempty projected record `ID`;
- output is sorted strictly by RuntimeInstance ID;
- projected core fields are converted to `loomruntime.RuntimeInstance` and
  revalidated through `loomruntime.NewRuntimeInstance`;
- canonical-form equality is required, including ordered copied observed
  capabilities;
- projected `ModelIDs` must be canonical: every ID is nonempty and strictly
  increasing;
- returned RuntimeInstances and capability slices are fresh copies; input or
  output mutation cannot change the other.

The S2-W22 baseline intentionally excludes projection-only model, digest,
source, and time metadata after that metadata has been validated.

## Discovery provenance validation

Every Runtime record must contain:

- a lowercase 64-character SHA-256 `DiscoveryDigest`;
- nonempty `SourceProbeID` and `DiscoveryID`;
- non-zero UTC `DiscoveredAt`;
- nonempty `DiscoveryEventID`; and
- positive `DiscoverySequence`.

Invalid or incomplete discovery/inventory provenance rejects the entire input.
The adapter does not attempt to reconstruct missing facts.

## Status provenance validation and selection

The ten S2-W24 status fields are one all-or-none group:

```text
StatusReconciliationID
StatusReconciliationDigest
StatusBaselineDigest
StatusDiscoveryDigest
StatusSourceProbeID
StatusChangedAt
StatusEventID
StatusSequence
StatusPreviousEventID
StatusPreviousSequence
```

When all ten fields have their zero values, the adapter selects
`DiscoveryEventID` / `DiscoverySequence`.

When any status field is non-zero, all ten must be complete and canonical:

- nonempty reconciliation ID, source probe ID, status Event ID, and previous
  Event ID;
- lowercase 64-character reconciliation, baseline, and source discovery
  digests;
- non-zero UTC status time;
- positive status and previous sequences;
- previous sequence is not `math.MaxInt64`;
- status sequence equals previous sequence plus one;
- previous sequence is greater than or equal to the current discovery sequence;
- previous Event ID equals the current discovery Event ID if and only if
  previous sequence equals the current discovery sequence; and
- status Event ID differs from its previous Event ID and from the current
  discovery Event ID.

Complete status metadata selects `StatusEventID` / `StatusSequence`.
The adapter does not claim to reconstruct an omitted historical chain from one
Snapshot record. Accepted S2-W24 replay remains authority for the Event-ID and
causation chain that produced that record.

A later accepted rediscovery has all ten status fields reset by S2-W24, so the
adapter selects that new discovery fact without retaining stale status
provenance.

## Cross-record Event identity

Accepted projection replay globally deduplicates Event IDs. The untrusted copied
Runtime map must therefore preserve one Runtime owner for every visible Event
identity.

Across different Runtime records, no Event ID may be reused in any role among:

- `DiscoveryEventID`;
- `StatusEventID` when status metadata is present; and
- `StatusPreviousEventID` when status metadata is present.

This is one combined identity set, so cross-role reuse across different Runtime
records is also rejected. Within one Runtime record, the only permitted alias is
the already frozen first-status relationship where
`StatusPreviousEventID == DiscoveryEventID` and the corresponding sequences are
equal. The existing rules already require `StatusEventID` to differ from both.

Any cross-record duplicate rejects the entire input with no partial baseline.
The adapter does not infer or repair ownership.

## Acceptance criteria

1. Empty/nil Runtime maps return a non-nil empty baseline.
2. Discovery-only records produce exact canonical RuntimeInstances with
   discovery Event provenance.
3. Complete one-step and consecutive status records produce status Event
   provenance rather than stale discovery provenance.
4. A rediscovered record with zero status metadata selects its new discovery
   provenance.
5. Multiple map entries produce deterministic strict ID order independent of
   map construction order and fresh calls.
6. Map-key mismatch, oversized input, invalid/noncanonical Runtime core,
   invalid models, invalid discovery metadata, partial/invalid status metadata,
   invalid sequence relationships, discovery/previous pair mismatch in either
   direction, any same-role or cross-role Event ID reuse across Runtime records,
   and invalid Event identity relationships fail closed with no partial
   baseline.
7. Input maps/slices/records and returned baseline/slices are mutation-isolated.
8. Renamed S2-W22 baselines preserve all accepted comparison, ordering,
   no-absence-inference, digest-sensitivity, Candidate immutability, and
   transition behavior.
9. The renamed baseline digest uses version `2`, binds generic previous Event
   ID/sequence semantics, and changes when either field changes.
10. Existing S2-W23 writer, S2-W24 projection, Runtime discovery/catalog,
    Journal, state, and repository behavior remains green.
11. No Snapshot mutation, Journal query/replay, Event construction/append,
    StateWriter, direct SQL/schema/config/file mutation, discovery or
    reconciliation invocation, status-from-absence inference, scheduler/
    daemon, Runtime selection/reservation/activation, process/model/network/
    environment/credential activity, external action, or Slice 3 behavior is
    introduced.

## Mandatory RED tests

Before product behavior changes, the Developer:

1. adds `internal/projection/runtime_status_baseline_test.go`;
2. updates the three accepted fixture files only for the frozen S2-W22 field
   rename and direct semantic proof; and
3. changes no product file.

The focused RED command must fail only because the frozen adapter/error symbols
and renamed baseline fields do not exist. No product implementation may precede
that observed failure.

Required groups named `TestBuildRuntimeStatusBaselines...`:

1. empty, discovery-only, one-step status, consecutive status, and rediscovery
   selection;
2. deterministic ordering, fresh-call equality, and input/output mutation
   isolation;
3. map key/count, Runtime core, model, discovery metadata, status all-or-none,
   digest/source/time/Event-ID, exact-next, discovery-order, and overflow
   failure matrices, including both forged directions where discovery and
   previous Event ID/sequence equality do not match as a pair;
4. all nine cross-record reuse combinations among discovery, current status,
   and previous status Event ID roles, with no partial output; and
5. static pure-adapter import/no-write/no-replay/no-discovery/
   no-reconciliation/no-execution/non-disclosure/scope assertions.

Renamed S2-W22 tests must directly prove that transitions use generic previous
status-bearing provenance and that the version-2 baseline digest binds both
renamed fields.

Tests use only direct copied Snapshot fixtures and existing deterministic fake
S2-W2 probes. They do not invoke Journal replay for adapter proof, Pi,
S2-W18/S2-W19 processes, installed Runtime state, network, credentials, package
managers, a scheduler, or a daemon.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/projection -run 'TestBuildRuntimeStatusBaselines'
  -count=1`
- Renamed S2-W22 regression:
  `go test ./internal/runtime -run 'TestReconcileObservedRuntimeStatuses'
  -count=1`
- Writer/projection regression:
  `go test ./internal/state ./internal/projection -run
  'Test(CommitRuntimeStatusTransitions|RebuildRuntimeStatusFacts|
  BuildRuntimeStatusBaselines)' -count=1`
- Package full:
  `go test ./internal/runtime ./internal/state ./internal/projection -count=1`
- Impact:
  `go test ./internal/runtime ./internal/state ./internal/projection
  ./internal/journal -count=1`
- Repeated focused race:
  `go test -race ./internal/runtime ./internal/state ./internal/projection -run
  'Test(ReconcileObservedRuntimeStatuses|CommitRuntimeStatusTransitions|
  RebuildRuntimeStatusFacts|BuildRuntimeStatusBaselines)' -count=30`
- Repository: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/runtime/status_reconciliation.go
  internal/runtime/status_reconciliation_test.go
  internal/state/runtime_status_writer_test.go
  internal/projection/runtime_status_test.go
  internal/projection/runtime_status_baseline.go
  internal/projection/runtime_status_baseline_test.go` and `git diff --check`
- Import boundary: new product code may import only the standard library plus
  accepted `internal/runtime`. It must not import Journal/state, concrete
  Runtime/Pi adapters, SQL, process, network, filesystem, config, credentials,
  scheduler, daemon, CLI/UI, or Slice 3 packages.
- Scope: only the frozen product/tests, S2-W25 evidence, `docs/CURRENT.md`, and
  Controller-owned non-historical `PROGRESS.md` changes may enter the
  Candidate.

## Explicit exclusions

No Event construction/append/StateWriter, Journal query/replay, projection
mutation/table/direct SQL/schema/config/file write, discovery or reconciliation
invocation, probe/Pi/S2-W18/S2-W19 process, installed Pi/user Pi state,
status-from-absence inference, RuntimeProfile selection, capacity reservation,
Runtime activation, Team/Agent/WorkItem/Run/Evidence/grant/resource mutation,
workspace, model, Bridge, claim, lease, scheduler, daemon/CLI/UI, network,
filesystem, environment, credential, production goroutine, external action,
push/merge/rebase/reset/release, or Slice 3 behavior.

VERDICT: CONTRACT_FROZEN
