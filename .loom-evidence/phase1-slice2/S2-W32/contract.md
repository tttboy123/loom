# S2-W32 Frozen WorkItem Contract

- ID: `S2-W32`
- Title: Discovery-Priority Runtime Observation Write Planning
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W31 local commit `b00f8d8`
- Corresponds to: `TECH-PLAN.md §6, §13.1, §14 Slice 2, §15.8`,
  ADR-0002, ADR-0003, accepted ADR-0007, and accepted
  S2-W2/S2-W25/S2-W31
- Frozen branch/head: `codex/loom-platform-slice2` at `b00f8d8`

## Owned files

Developer-owned:

- `internal/app/runtime_write_plan.go`
- `internal/app/runtime_write_plan_test.go`
- `.loom-evidence/phase1-slice2/S2-W32/deliverable.md`

Controller-owned:

- `docs/adr/0007-runtime-observation-discovery-priority.md`
- `docs/adr/README.md`
- `.loom-evidence/phase1-slice2/S2-W32/contract.md`
- `.loom-evidence/phase1-slice2/S2-W32/contract-review.md`
- `docs/CURRENT.md`
- Controller-owned non-historical `PROGRESS.md` changes

All accepted Slice 1 and S2-W1 through S2-W31 product/test files remain
unchanged. Any ownership expansion requires a recorded amendment and fresh
contract Reviewer `PASS`.

## Objective

Add one pure deterministic Candidate boundary that classifies a caller-supplied
accepted Runtime observation as exactly one write kind under accepted ADR-0007:

1. `discovery` when any present Runtime is new or any observed non-status
   inventory changed;
2. `status` only when inventory is unchanged and at least one present Runtime
   status changed; or
3. `none` when no present observed fact changed.

`discovery` has strict precedence over `status`. Missing projected Runtime
records never create a change. The planner validates but does not invoke S2-W27
or S2-W31, write an Event, prepare metadata, schedule, start a daemon, infer
absence, or activate a Runtime.

## Frozen public boundary

Same accepted package: `internal/app`.

```go
type RuntimeObservationWriteKind string

const (
    RuntimeObservationWriteNone      RuntimeObservationWriteKind = "none"
    RuntimeObservationWriteDiscovery RuntimeObservationWriteKind = "discovery"
    RuntimeObservationWriteStatus    RuntimeObservationWriteKind = "status"
)

var ErrInvalidRuntimeObservationWritePlan = errors.New(
    "invalid runtime observation write plan",
)

type RuntimeObservationWritePlanCandidate struct {
    // private immutable facts
}

func PlanRuntimeObservationWrite(
    ctx context.Context,
    projected projection.Snapshot,
    current runtime.RuntimeDiscoverySnapshot,
) (RuntimeObservationWritePlanCandidate, error)

func (c RuntimeObservationWritePlanCandidate) Planned() bool
func (c RuntimeObservationWritePlanCandidate) Kind() RuntimeObservationWriteKind
func (c RuntimeObservationWritePlanCandidate) SourceDiscoveryDigest() string
func (c RuntimeObservationWritePlanCandidate) ProjectedRuntimeCount() int
func (c RuntimeObservationWritePlanCandidate) ObservedRuntimeCount() int
func (c RuntimeObservationWritePlanCandidate) InventoryChangeCount() int
func (c RuntimeObservationWritePlanCandidate) StatusTransitionCount() int
func (c RuntimeObservationWritePlanCandidate) CandidateDigest() string
```

The product imports only standard library plus accepted
`internal/projection` and `internal/runtime`.

## Validation and trusted composition

- Nil context returns the zero Candidate plus
  `ErrInvalidRuntimeObservationWritePlan`.
- Canceled/deadline context propagates canonically before work.
- Call accepted `projection.BuildRuntimeStatusBaselines(projected)` once.
  Its complete projection validation and exact errors remain authoritative.
- Check context, then call accepted
  `runtime.ReconcileObservedRuntimeStatuses(ctx, baseline, current)` once.
  Its discovery validation, stable identity drift, no-absence semantics,
  status-transition detection, and exact errors remain authoritative.
- Check context before and during classification and before success.
- Every error returns the zero Candidate.
- Inputs are caller-supplied copied values and are neither retained nor
  mutated.

The planner does not duplicate or weaken Event provenance, map/observation
bounds, canonical Runtime/model validation, identity validation, deterministic
ordering, digest validation, or mutation isolation owned by S2-W2/S2-W25/
S2-W22.

## Inventory comparison

For every current observation whose Runtime ID exists in the projection:

- `DeviceID` or `AdapterType` mismatch is already rejected by accepted S2-W22;
- compare exact canonical `DisplayName`, `ExecutableVersion`,
  `ObservedCapabilities`, `Capacity`, `ModelIDs`, and `SourceProbeID`;
- do not compare `Status`, discovery/status digests, Event IDs, sequences,
  correlation IDs, or timestamps as inventory; and
- do not treat a projected Runtime missing from current observation as a
  change.

A current-only Runtime contributes one inventory change. Each present Runtime
contributes at most one inventory change even if several inventory fields
differ.

## Discovery-priority classification

After complete validation:

```text
if InventoryChangeCount > 0:
    Kind = discovery
else if StatusTransitionCount > 0:
    Kind = status
else:
    Kind = none
```

The status-transition count is the accepted S2-W22 Candidate count, including
in a mixed observation. It is evidence only; `Kind=discovery` forbids a second
status write for that observation.

Examples:

- empty projection + empty accepted observation: `none`;
- non-empty projection + empty accepted observation: `none`;
- current-only Runtime: `discovery`;
- unchanged present Runtime plus absent projected Runtime: `none`;
- status-only change plus absent projected Runtime: `status`;
- one inventory change plus any number of status changes: `discovery`;
- stable identity drift: error, never `discovery`.

## Candidate validity and digest

Every successful result is a nonzero immutable Candidate with:

- `Planned() == true`;
- one exact known kind;
- exact source discovery digest;
- projected and observed counts in `[0, 32]`;
- inventory/status counts in `[0, observed count]`;
- classification consistent with the precedence rule; and
- lowercase SHA-256 `CandidateDigest`.

The digest uses one canonical versioned payload containing all exposed facts.
Equivalent canonical projection maps and observations produce the same digest
regardless of map construction order. Any exposed semantic fact change changes
the digest. The zero Candidate exposes only false, empty, and zero values.

## Acceptance criteria

1. Nil/canceled/deadline context returns the exact error and zero Candidate.
2. Empty, absent-only, and unchanged observations produce exact valid `none`
   Candidates without absence inference.
3. Current-only Runtime and every non-status inventory field change produce
   `discovery`.
4. One/multiple status-only changes produce `status`.
5. Mixed inventory/status changes produce `discovery` while retaining the
   exact nonzero status-transition count as audit evidence.
6. Invalid/oversized projection and invalid discovery propagate their accepted
   exact errors with zero Candidate.
7. Stable identity drift propagates the accepted exact error and never becomes
   rediscovery.
8. Candidate facts, precedence, digest determinism/sensitivity, input
   immutability, and zero-value behavior are complete and non-hollow.
9. Existing S2-W2, S2-W20 through S2-W31, Runtime, projection, app, and
   repository behavior remains green.
10. No writer/committer call, Journal/SQLite/projection rebuild, Event metadata,
    retry, configuration/file/process/network access, scheduler/ticker/sleep/
    goroutine/daemon, status-from-absence inference, RuntimeProfile selection,
    capacity reservation, Runtime activation, Team/Agent/Work/Run/Grant/Bridge,
    credential, external action, or Slice 3 behavior is introduced.

## Mandatory RED tests

Before product code, add only `internal/app/runtime_write_plan_test.go`.
Focused RED must fail only because the frozen symbols are missing.

Required groups named `TestPlanRuntimeObservationWrite...`:

1. nil/canceled/deadline context and zero Candidate;
2. empty, unchanged, absent-only, and present-status-plus-absent cases;
3. current-only and exact per-field inventory-change table;
4. one/multiple status-only and mixed discovery-priority cases;
5. invalid/oversized projection, invalid discovery, and stable identity drift;
6. deterministic digest, semantic sensitivity, map-order independence,
   zero-value accessors, and input mutation isolation; and
7. static import/no-writer/no-Journal/no-metadata/no-scheduler/no-daemon/
   no-execution-authority/scope assertions.

Tests use deterministic copied domain/projection Candidates only. They do not
use SQLite, S2-W18, installed Pi, user Pi state, credentials, network, process,
filesystem, scheduler, daemon, model call, Runtime activation, or external
action.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/app -run 'TestPlanRuntimeObservationWrite' -count=1`
- App package: `go test ./internal/app -count=1`
- Impact:
  `go test ./internal/app ./internal/runtime ./internal/projection -count=1`
- Repeated focused race:
  `go test -race ./internal/app -run
  'TestPlanRuntimeObservationWrite' -count=50`
- Repository: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/app/runtime_write_plan.go
  internal/app/runtime_write_plan_test.go` and `git diff --check`
- Documentation: ADR-0007 is indexed, accepted, link-resolving, and consistent
  with TECH-PLAN Slice 2 and the frozen product boundary.
- Scope: only frozen files, S2-W32 evidence, `docs/CURRENT.md`, and
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

No writer/committer invocation, Journal query/rebuild/SQLite, Event/payload/
metadata preparation or allocation, retry, unified sequence allocation,
discovery/status dual-write, absence-based status/deletion, scheduler/ticker/
sleep/goroutine/daemon/config entry, file/process/network/environment access,
installed Runtime lookup, Pi policy, RuntimeProfile selection, capacity
reservation, Runtime activation, Team/Agent/WorkItem/Run/Evidence/grant/
resource mutation, workspace, model, Bridge, claim, lease, credential, CLI/UI,
external action, Phase 2, or Slice 3 behavior.

VERDICT: CONTRACT_FROZEN
