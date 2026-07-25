# S2-W21 Frozen WorkItem Contract

- ID: `S2-W21`
- Title: Runtime Discovery Read-Model Projection
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W20 and local S2-W20 commit `501ac33`
- Corresponds to: `TECH-PLAN.md §6, §13.1, §14 Slice 2, §15.8`,
  ADR-0002, and ADR-0003
- Frozen branch/head: `codex/loom-platform-slice2` at `501ac33`

## Owned files

- `internal/projection/projection.go`
- `internal/projection/projection_test.go`
- `internal/projection/runtime_discovery.go`
- `internal/projection/runtime_discovery_test.go`
- `.loom-evidence/phase1-slice2/S2-W21/deliverable.md`

Accepted S1-W4 and S2-W16 projection behavior remains authoritative and must
stay green. This WorkItem explicitly reopens only the accepted projection
product/test files for the bounded Runtime discovery read-model integration.
Changes to the two accepted files are limited to:

- one `RuntimeInstances` Snapshot map;
- one `RuntimeInstanceDiscovered` apply dispatch;
- initialization and deep-copy integration for that map; and
- adding the new empty map to the two legacy literal Snapshot expectations.

All validation and projection logic belongs in the new Runtime-specific files.
Other accepted Slice 1 and S2-W1 through S2-W20 product/test files remain
unchanged. Any ownership expansion requires a recorded Controller amendment and
fresh contract Reviewer `PASS`.

## Objective

Extend the accepted rebuildable in-memory projection so committed canonical
S2-W20 `RuntimeInstanceDiscovered` Events become queryable inventory:

1. replay one exact immutable record per RuntimeInstance ID;
2. expose the latest valid discovery fact in that instance stream;
3. preserve stable device and adapter identity across rediscovery;
4. defensively revalidate the complete Event envelope, payload, RuntimeInstance,
   discovery digest, model inventory, and canonical ordering; and
5. deep-copy every map and mutable slice on Snapshot boundaries.

This WorkItem reads committed Journal facts only. It never runs discovery,
appends Events, writes a projection table, infers status from absence, applies
`RuntimeInstanceStatusChanged`, chooses a RuntimeProfile, reserves capacity,
starts a process, or activates a Runtime.

## Frozen read-model shape

`Snapshot` adds:

```go
RuntimeInstances map[string]RuntimeInstance
```

The projection-local immutable view is:

```go
type RuntimeInstance struct {
    ID                   string
    DeviceID             string
    AdapterType          string
    DisplayName          string
    ExecutableVersion    string
    Status               string
    ObservedCapabilities []string
    Capacity             int
    ModelIDs             []string
    DiscoveryDigest      string
    SourceProbeID        string
    DiscoveryID          string
    DiscoveredAt         time.Time
    DiscoveryEventID     string
    DiscoverySequence    int64
}
```

No executable/search/isolation path, file identity/digest, environment,
stdout/stderr, credential, prompt, session, user Pi state, mutable source
object, process handle, or activation authority enters the read model.

`emptySnapshot` always returns six nonnil maps. `Snapshot.clone` deep-copies
the Runtime map plus every `ObservedCapabilities` and `ModelIDs` slice.

## Authoritative input

The only new relevant Event is schema-v1 `RuntimeInstanceDiscovered`, emitted
by accepted S2-W20. Its exact payload is:

```json
{
  "discovery_digest": "<lowercase sha256>",
  "source_probe_id": "<nonempty>",
  "instance": {
    "id": "<nonempty>",
    "device_id": "<nonempty>",
    "adapter_type": "<nonempty>",
    "display_name": "<nonempty>",
    "executable_version": "<string, may be empty>",
    "status": "<accepted RuntimeStatus>",
    "observed_capabilities": ["<canonical capability>"],
    "capacity": 1
  },
  "model_ids": ["<canonical model id>"]
}
```

The projection must decode with unknown-field rejection. It revalidates the
nested instance through accepted `runtime.NewRuntimeInstance`; it does not
become a second Runtime catalog authority. Model IDs may be empty, but every
present ID is nonempty, strictly increasing, and unique, matching accepted
S2-W2/S2-W20 canonical form. `ExecutableVersion` is projected exactly and may
be empty because the accepted RuntimeInstance contract permits it.

## Event envelope validation

Every relevant Event requires:

- nonempty Event ID, idempotency key, and correlation ID;
- schema version `1`, positive sequence, and type
  `RuntimeInstanceDiscovered`;
- nonzero `EmittedAt` whose location is exactly `time.UTC`;
- empty causation ID;
- stream exactly `runtime_instance:<payload.instance.id>`;
- lowercase 64-character discovery digest;
- nonempty source probe ID; and
- a payload RuntimeInstance whose ID equals the Snapshot map key.

Malformed JSON, missing fields, unknown fields, invalid RuntimeInstance
content, noncanonical capabilities/models, invalid digest, envelope mismatch,
or inconsistent stream/instance identity fails closed with
`ErrInvalidProjectionEvent`.

The accepted replay layer continues to own Event ID conflict, stream sequence,
idempotent duplicate, schema-version, ordering, cancellation, and atomic
Snapshot-swap behavior.

Event ID, idempotency key, and correlation ID are caller-owned S2-W20 metadata.
The projector has authority to require that they are nonempty and to preserve
the correlation as `DiscoveryID`; accepted Journal/replay logic has authority
over reused-ID/key conflicts and stream sequences. No payload field binds an
exact expected value for otherwise fresh nonempty metadata. Alternate nonempty
unique Event IDs, idempotency keys, and correlation IDs are therefore valid
and must not be rejected.

## Rediscovery semantics

The first valid Event creates one read-model record. A later valid Event in the
same instance stream replaces the current observed fields and source metadata
with the higher-sequence fact.

Across rediscovery:

- `ID`, `DeviceID`, and `AdapterType` are stable identity fields;
- changing `DeviceID` or `AdapterType` fails closed;
- display name, executable version, accepted status, capabilities, capacity,
  models, source probe, discovery digest/ID/time/Event ID, and sequence may
  change to the latest valid fact; and
- absence of an Event never deletes a record or fabricates offline,
  incompatible, disabled, or any other status.

Exact duplicate Events remain handled by accepted replay deduplication.
`RuntimeInstanceStatusChanged` remains an unknown-event no-op until a separate
frozen WorkItem defines historical reconciliation and status transition
semantics.

## Existing projection behavior

- Existing `Modes`, `WorkItems`, `Evidence`, `Teams`, and `AgentInstances`
  behavior remains unchanged.
- Unknown Event types remain explicit no-ops after normal replay sequencing.
- Reordered Journal input, exact duplicate Events, conflict/gap/version checks,
  cancellation, concurrent rebuild serialization, Journal failure, and
  prior-Snapshot preservation remain unchanged.
- Saved Team/Main cross-record validation remains unchanged.
- Equal Journal content always rebuilds an equal Snapshot.

## Acceptance criteria

1. A real SQLite Journal containing an exact S2-W20 multi-instance batch
   rebuilds the complete Runtime map, and a new Projection rebuilds an equal
   Snapshot.
2. Input/insertion order does not affect the Snapshot; map keys and every
   projected scalar/slice/source/Event field are exact.
3. Empty Journal and unrelated Events produce a nonnil empty Runtime map and
   no inferred status.
4. A higher-sequence rediscovery replaces all allowed observed/source fields.
5. Rediscovery with changed device or adapter identity fails closed and
   preserves the prior Snapshot.
6. Missing, malformed, extra, invalid-runtime, invalid-capability/model,
   invalid-digest, and invalid-envelope facts fail closed.
7. Wrong stream, nonempty causation, non-UTC/zero timestamp, empty Event ID,
   empty idempotency key, empty correlation ID, or invalid sequence fails
   closed through the applicable accepted replay/projector authority. Fresh
   alternate nonempty unique caller-owned Event IDs, keys, and correlations
   remain valid.
8. Failed, canceled, concurrent, or closed-Journal rebuild never partially
   swaps Runtime or existing maps.
9. Caller mutation of returned maps, Runtime values, capabilities, or models
   cannot change the stored Snapshot.
10. Existing S1-W4 and S2-W16 behavior plus all other accepted Slice 1/Slice 2
    behavior remains green.
11. No Event append, direct SQL projection write, migration/schema change,
    discovery/probe/process/model/network/filesystem/environment activity,
    RuntimeProfile selection, capacity reservation, activation, Team/Agent/
    WorkItem/Run/grant mutation, daemon/CLI/UI, external action, or Slice 3
    behavior is introduced.

## Mandatory RED tests

The Developer first adds the new Runtime-focused test file and only the two
frozen empty-map legacy expectation edits. RED must fail on missing
`RuntimeInstances`, `RuntimeInstance`, and discovery apply behavior only.

Required groups named `TestRebuildRuntimeDiscoveryFacts...`:

1. real SQLite replay of an exact accepted S2-W20 multi-instance batch and new
   Projection rebuild equivalence;
2. exact projected shape, order independence, empty/unrelated behavior, and no
   absence/status inference;
3. higher-sequence rediscovery plus immutable-identity drift rejection;
4. payload, RuntimeInstance, capability, model, digest, and envelope failure
   matrices, including empty caller metadata and acceptance of alternate
   nonempty unique caller metadata;
5. failed/canceled/concurrent/closed-Journal prior-Snapshot preservation;
6. map/value/capability/model mutation isolation; and
7. legacy behavior plus static import, non-disclosure, and trust-boundary
   assertions.

Tests may use deterministic fake S2-W2 probes and the accepted S2-W20 writer
against temporary SQLite. They must not invoke S2-W18/S2-W19, an installed Pi,
user Pi state, network, credentials, package managers, or a long-lived Runtime.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/projection -run 'TestRebuildRuntimeDiscoveryFacts'
  -count=1`
- Package full: `go test ./internal/projection -count=1`
- Impact:
  `go test ./internal/projection ./internal/state ./internal/runtime
  ./internal/journal -count=1`
- Repeated focused race:
  `go test -race ./internal/projection -run
  'TestRebuildRuntimeDiscoveryFacts' -count=30`
- Repository: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/projection/projection.go
  internal/projection/projection_test.go
  internal/projection/runtime_discovery.go
  internal/projection/runtime_discovery_test.go` and `git diff --check`
- Import boundary: `projection.go` remains standard library plus accepted
  `internal/journal`; `runtime_discovery.go` may additionally import accepted
  `internal/runtime`. Production must not import `internal/state`, concrete
  runtime adapters, SQL drivers, process/network/config/credential/scheduler/
  daemon/UI packages, or Slice 3 surfaces.
- Scope: `git diff --name-only 501ac33` contains only the frozen product/test,
  S2-W21 evidence, `docs/CURRENT.md`, and Controller-owned non-historical
  `PROGRESS.md` paths.

## Explicit exclusions

No Event append, projection-table or direct SQL write, migration/schema change,
discovery execution, installed Pi access, executable/search/isolation path,
file identity/digest, environment, output, credentials, prompts, sessions,
user Pi state, status-from-absence inference, `RuntimeInstanceStatusChanged`
handling, RuntimeProfile selection, capacity reservation, Runtime activation,
Team/Agent/WorkItem/Run/Evidence/grant/resource mutation, workspace, process,
model, Bridge, claim, lease, scheduler, daemon/CLI/UI, network, filesystem,
production goroutine, external action, or Slice 3 behavior.

VERDICT: CONTRACT_FROZEN
