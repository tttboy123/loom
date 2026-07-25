# S2-W24 Frozen WorkItem Contract

- ID: `S2-W24`
- Title: Runtime Status Event Projection
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W23 local commit `1ba3238`
- Corresponds to: `TECH-PLAN.md §6, §13.1, §14 Slice 2`,
  ADR-0002, ADR-0003, and accepted S2-W20/S2-W21/S2-W22/S2-W23
- Frozen branch/head: `codex/loom-platform-slice2` at `1ba3238`

## Owned files

- `internal/projection/projection.go`
- `internal/projection/projection_test.go`
- `internal/projection/runtime_discovery.go`
- `internal/projection/runtime_discovery_test.go`
- `internal/projection/runtime_status.go`
- `internal/projection/runtime_status_test.go`
- `.loom-evidence/phase1-slice2/S2-W24/deliverable.md`

The Developer owns only these files. Controller-owned contract, review, status,
and documentation evidence remain outside Developer ownership. All other
accepted Slice 1 and S2-W1 through S2-W23 product/test files remain unchanged.
Any ownership expansion requires a recorded Controller amendment and fresh
contract Reviewer `PASS`.

`projection.go`, `projection_test.go`, `runtime_discovery.go`, and
`runtime_discovery_test.go` are accepted files. Changes are limited to Event
dispatch, the frozen Runtime read-model shape, status/discovery interaction, and
direct regression proof required by this contract.

## Objective

Extend the accepted rebuildable projection with canonical
`RuntimeInstanceStatusChanged` handling:

1. consume only committed Journal Events through accepted replay;
2. require an already projected Runtime discovery fact;
3. validate the complete S2-W23 envelope and exact payload;
4. require stable Runtime identity, exact current `from_status`, exact previous
   status-bearing Event provenance, and exact-next sequence/causation;
5. update only the Runtime status plus copied status-transition provenance; and
6. preserve atomic rebuild, deterministic replay, snapshot isolation, discovery
   provenance, and all non-status inventory facts.

This WorkItem does not run discovery/reconciliation, append Events, write a
projection table or direct SQL, build the next S2-W22 baseline, allocate stream
metadata, infer status from absence, schedule scans, start a daemon, select or
reserve a Runtime, activate execution, or invoke any process/model/Agent.

## Frozen read-model extension

The accepted projection `RuntimeInstance` adds:

```go
StatusReconciliationID     string
StatusReconciliationDigest string
StatusBaselineDigest       string
StatusDiscoveryDigest      string
StatusSourceProbeID        string
StatusChangedAt            time.Time
StatusEventID              string
StatusSequence             int64
StatusPreviousEventID      string
StatusPreviousSequence     int64
```

Existing fields remain unchanged:

```text
ID, DeviceID, AdapterType, DisplayName, ExecutableVersion, Status,
ObservedCapabilities, Capacity, ModelIDs, DiscoveryDigest, SourceProbeID,
DiscoveryID, DiscoveredAt, DiscoveryEventID, DiscoverySequence
```

Before any status Event, every added field is its zero value. Snapshot access
continues to return copied Runtime records and copied capability/model slices.

### Status-bearing fact provenance

For projection validation only, the latest status-bearing fact is:

- `StatusEventID` / `StatusSequence` when nonempty/positive; otherwise
- `DiscoveryEventID` / `DiscoverySequence`.

A status Event updates the added status provenance while preserving every
discovery and inventory field. A later accepted `RuntimeInstanceDiscovered`
Event keeps accepted S2-W21 complete-rediscovery semantics: it replaces the
complete observed inventory/status/discovery record and resets every added
status-transition field to zero. The latest status-bearing fact is then the new
discovery Event.

This WorkItem does not change the frozen S2-W22 baseline type or decide how a
future scheduler/baseline adapter selects provenance for a new reconciliation.
That integration remains a separate frozen boundary.

## Frozen Event validation

`RuntimeInstanceStatusChanged` is no longer an unknown no-op. The handler
requires:

- nonempty Event ID, idempotency key, correlation ID, and causation ID;
- positive sequence;
- non-zero UTC `EmittedAt`;
- exact schema version `1`;
- exact stream `runtime_instance:<runtime_instance_id>`;
- an existing projected Runtime record;
- exact JSON with all and only the frozen payload fields;
- lowercase 64-character SHA-256 reconciliation, baseline, and source
  discovery digests;
- nonempty source probe, Runtime ID, device ID, adapter type, and previous Event
  ID;
- positive previous sequence;
- accepted and distinct `from_status` / `to_status`;
- device/adapter equality with the existing Runtime record;
- `from_status` equal to the currently projected status;
- previous Event ID/sequence equal to the latest projected status-bearing fact;
- Event `CausationID` equal to payload `previous_event_id`; and
- Event sequence equal to `previous_sequence + 1`, with overflow rejected.

Frozen payload:

```text
reconciliation_digest
baseline_digest
source_discovery_digest
source_probe_id
runtime_instance_id
device_id
adapter_type
from_status
to_status
previous_event_id
previous_sequence
```

Missing, null, duplicate, or unknown fields; malformed JSON; invalid digests or
statuses; unknown Runtime; identity drift; current-status mismatch; stale or
future previous provenance; causation mismatch; non-UTC time; or non-exact-next
sequence fails closed with `ErrInvalidProjectionEvent`.

Accepted replay continues to own Event-ID/stream-sequence conflicts, exact
duplicate deduplication, schema-version rejection, ordering, cancellation,
sequence-gap detection, and atomic Snapshot swap. The status handler revalidates
its own envelope/payload contract and never weakens replay authority.

## Projection semantics

For one valid status Event:

- all inventory and discovery fields remain byte/value equal;
- `Status` changes exactly from payload `from_status` to `to_status`;
- added status fields copy Event correlation/digests/source/time/ID/sequence and
  payload previous provenance exactly.

A valid consecutive status Event is accepted only when its `from_status`,
previous Event ID/sequence, causation, and exact-next sequence chain from the
already projected status Event. A repeated or stale fact referencing an older
discovery/status Event fails closed.

A later valid rediscovery retains accepted S2-W21 behavior, including a changed
accepted status or other observed field, and clears all added status-transition
metadata. Absence of any Event never deletes a Runtime or fabricates a status.
Unrelated Event types remain no-ops after accepted replay validation.

## Acceptance criteria

1. Real Journal replay of accepted S2-W20 discovery plus S2-W23 status Events
   rebuilds the exact status and provenance while preserving inventory and
   discovery fields.
2. Rebuild is deterministic across Event input order and fresh Projection
   instances.
3. Valid consecutive status Events chain from the latest status-bearing fact.
4. A later valid rediscovery replaces complete observed facts and resets all
   added status-transition metadata.
5. Status before discovery, unknown Runtime, stable-identity drift, current
   from-status mismatch, equal/unknown statuses, stale/future previous
   provenance, causation mismatch, lower/equal/higher sequence, invalid
   envelope/payload/digest/source/time, missing/null/unknown/duplicate fields,
   and malformed JSON fail closed.
6. Existing Snapshot remains unchanged after every failed rebuild.
7. Exact duplicate replay remains idempotent; replay sequence gaps and Event
   conflicts retain their accepted errors.
8. Snapshot, source Event, payload, capability, model, and returned-record
   mutation cannot alter committed projection state.
9. Empty Journal input retains an empty Runtime map; absence never deletes or
   changes a known Runtime.
10. Existing discovery rediscovery, identity, empty/unrelated, saved-Team, core
    projection, Journal, Runtime, and S2-W20 through S2-W23 behavior remains
    green.
11. No Event append, discovery/reconciliation invocation, next-baseline
    construction, projection-table/direct SQL/schema/config/file mutation,
    status-from-absence inference, scheduler/daemon, Runtime selection/
    reservation/activation, process/model/network/environment/credential
    activity, external action, or Slice 3 behavior is introduced.

## Mandatory RED tests

Before product behavior changes, the Developer:

1. adds `internal/projection/runtime_status_test.go`; and
2. updates only the accepted S2-W21 test that currently treats
   `RuntimeInstanceStatusChanged` as an unrelated unknown Event so it instead
   verifies a genuinely unrelated Event remains a no-op.

The focused RED command must fail only because the frozen status read-model
fields/handler behavior do not exist; no product implementation may precede it.

Required groups named `TestRebuildRuntimeStatusFacts...`:

1. real S2-W20 + S2-W22 + S2-W23 Journal integration and exact projected
   status/provenance/inventory preservation;
2. deterministic order, fresh rebuild, exact duplicate, empty/absence, and
   snapshot/source/accessor mutation isolation;
3. consecutive status chain and rediscovery reset semantics;
4. status-before-discovery, unknown Runtime, identity/from-status/status/
   previous/causation/sequence failure matrices with preserved Snapshot;
5. exact envelope and JSON malformed/missing/null/unknown/duplicate-field,
   digest, source, time, stream, and schema validation;
6. accepted replay conflict/gap/cancellation behavior remains intact; and
7. static import/no-write/no-discovery/no-reconciliation/no-execution/
   non-disclosure/scope assertions.

Tests use deterministic fake S2-W2 probes, accepted StateWriters, temporary
SQLite, and direct immutable Event fixtures only. They do not invoke Pi,
S2-W18/S2-W19 processes, installed Runtime state, network, credentials, package
managers, a scheduler, or a daemon.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/projection -run 'TestRebuildRuntimeStatusFacts' -count=1`
- Package full: `go test ./internal/projection -count=1`
- Impact:
  `go test ./internal/projection ./internal/state ./internal/runtime
  ./internal/journal -count=1`
- Repeated focused race:
  `go test -race ./internal/projection -run
  'TestRebuildRuntimeStatusFacts' -count=30`
- Repository: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/projection/projection.go
  internal/projection/projection_test.go
  internal/projection/runtime_discovery.go
  internal/projection/runtime_discovery_test.go
  internal/projection/runtime_status.go
  internal/projection/runtime_status_test.go` and `git diff --check`
- Import boundary: new product code may import only the standard library plus
  accepted `internal/journal` and `internal/runtime`. Projection product must not
  import StateWriter, concrete Runtime/Pi adapters, process, network,
  filesystem, config, credentials, scheduler, daemon, CLI/UI, or Slice 3
  packages.
- Scope: only the frozen product/tests, S2-W24 evidence, `docs/CURRENT.md`, and
  Controller-owned non-historical `PROGRESS.md` changes may enter the
  Candidate.

## Explicit exclusions

No Event append/StateWriter, discovery or reconciliation invocation, S2-W22
baseline mutation/construction, projection-table/direct SQL/schema/config/file
write, absence inference, installed Pi access, executable/search/isolation
path, environment/output/credentials/prompts/sessions/user Pi state,
RuntimeProfile selection, capacity reservation, Runtime activation, Team/
Agent/WorkItem/Run/Evidence/grant/resource mutation, workspace, process, model,
Bridge, claim, lease, scheduler, daemon/CLI/UI, network, filesystem, production
goroutine, external action, push/merge/release, or Slice 3 behavior.

VERDICT: CONTRACT_FROZEN
