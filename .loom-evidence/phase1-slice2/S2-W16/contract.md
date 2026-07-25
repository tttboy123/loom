# S2-W16 Frozen WorkItem Contract

- ID: `S2-W16`
- Title: Team and Agent Instance Read-Model Projection
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W15 and local S2-W15 commit `560834a`
- Corresponds to: `PRODUCT-PLAN.md §4, §6`, `TECH-PLAN.md §4, §6,
  §13.1, §14 Slice 2, §15.2, §15.5, §15.18, §15.21`, ADR-0001,
  ADR-0002, and ADR-0003
- Frozen branch/head: `codex/loom-platform-slice2` at `560834a`

## Owned files

- `internal/projection/projection.go`
- `internal/projection/projection_test.go`
- `.loom-evidence/phase1-slice2/S2-W16/deliverable.md`

Accepted S1-W4 projection behavior remains authoritative and must stay green.
This WorkItem explicitly reopens only `internal/projection/projection.go` and
`internal/projection/projection_test.go` as a bounded S1-W4 read-model
extension. Other accepted Slice 1 and S2-W1 through S2-W15 product/test files
remain unchanged. Any further ownership amendment requires a recorded
Controller amendment and fresh contract Reviewer PASS.

## Objective

Extend the accepted rebuildable in-memory projection so the two authoritative
S2-W15 facts become queryable after Journal replay:

1. project `TeamInstanceCreated` into one immutable TeamInstance read-model
   record;
2. project Main `AgentInstanceCreated` into one immutable AgentInstance
   read-model record;
3. validate their envelope, canonical schema-v1 payload, scope, digests,
   counts, Runtime binding, correlation, causation, and cross-record links;
4. require every projected saved Team to have exactly one linked Main Agent;
   and
5. deep-copy maps and dormant-member slices on every Snapshot boundary.

This WorkItem reads committed facts only. It does not append Events, write a
projection table, change schema, instantiate resources, create WorkItems/Runs/
grants/workspaces/processes, reserve capacity, or execute.

## Frozen read-model shape

`Snapshot` adds:

```go
Teams          map[string]TeamInstance
AgentInstances map[string]AgentInstance
```

The projection-local immutable view types contain only scalar fields and copied
slices:

```go
type ScopeIdentity struct {
    ProjectID    string
    GenerationID string
}

type DormantSubAgent struct {
    Dormant           bool
    AgentDefinitionID string
    RuntimeProfileID  string
    RuntimeInstanceID string
}

type TeamInstance struct {
    ID                    string
    WorkRequestID         string
    SourceKind            string
    TeamDefinitionID      string
    TeamDefinitionVersion int
    TeamDefinitionScope   string
    ScopeIdentity         ScopeIdentity
    TeamDefinitionDigest  string
    SourcePlanDigest      string
    SourceRecordSetDigest string
    State                 string
    CreatedAt             int64
    DormantSubAgents      []DormantSubAgent
    TeamInstanceCount     int
    AgentInstanceCount    int
    ActiveSubAgentCount   int
    WorkItemCount         int
    CreationEventID       string
}

type RuntimeBinding struct {
    Accepted   bool
    ProfileID  string
    InstanceID string
}

type AgentInstance struct {
    ID                     string
    TeamInstanceID         string
    WorkRequestID          string
    AgentDefinitionID      string
    AgentDefinitionVersion int
    AgentDefinitionScope   string
    ScopeIdentity          ScopeIdentity
    RuntimeProfileID       string
    RuntimeInstanceID      string
    IsMain                 bool
    State                  string
    RuntimeBinding         RuntimeBinding
    SourcePlanDigest       string
    SourceRecordSetDigest  string
    TeamCreatedAt          int64
    BindingDigest          string
    RuntimeDiscoveryDigest string
    CreationEventID        string
    TeamCreationEventID    string
}
```

No Agent definition content, prompt, raw user text, credentials, environment,
mutable source object, or execution handle enters the read model.

## Team fact validation

For `TeamInstanceCreated`, replay requires:

- schema version `1`, sequence `1`, nonempty Event ID and correlation ID,
  empty causation ID;
- stream exactly `team_instance:<payload.team.id>`;
- correlation exactly `payload.team.work_request_id`;
- exact schema-v1 snake_case payload fields emitted by S2-W15;
- `source_kind="saved_team"`, `state="created"`, positive Team definition
  version and `created_at`;
- project scope with nonempty project ID and empty generation ID, or reusable
  scope with both IDs empty;
- valid lowercase SHA-256 Team definition, source plan, and record-set digests;
- nested and outer source plan digests equal;
- counts exactly Team `1`, Agent `1`, active SubAgent `0`, WorkItem `0`;
- zero through two dormant entries, each `dormant=true` with nonempty Agent,
  RuntimeProfile, and RuntimeInstance IDs;
- dormant entries in strict Agent/Profile/Instance lexical order with no
  duplicate Agent definition ID.

Any missing, malformed, unknown, inconsistent, or out-of-range relevant field
fails closed with `ErrInvalidProjectionEvent`.

## Main Agent fact validation

For `AgentInstanceCreated`, replay requires:

- schema version `1`, sequence `1`, nonempty Event ID, correlation ID, and
  causation ID;
- stream exactly `agent_instance:<payload.main_agent.id>`;
- exact schema-v1 snake_case payload fields emitted by S2-W15;
- nonempty Team/Agent/Runtime IDs, positive Agent definition version and Team
  creation timestamp;
- `is_main=true`, `state="created"`;
- project Agent scope with nonempty project ID and empty generation ID, or
  reusable Agent scope with both IDs empty;
- accepted Runtime binding whose profile/instance IDs exactly equal the Main
  Agent record;
- valid lowercase SHA-256 source plan, record-set, binding, and Runtime
  discovery digests.

After all streams replay, every saved Team requires exactly one Main Agent and
every projected Agent requires its Team. The linked records must have identical
WorkRequest ID/correlation, source plan digest, record-set digest, and Team
creation timestamp. The Agent causation ID must equal the Team creation Event
ID. Orphans, missing Main Agents, multiple Main Agents, or link mismatches fail
closed without swapping the prior Snapshot.

Because replay sorts by stream, the Main Agent stream may be applied before its
Team stream. Referential validation occurs only after all Events have been
individually validated and applied.

## Existing projection behavior

- Existing `Modes`, `WorkItems`, and `Evidence` behavior remains unchanged.
- Unknown Event types remain explicit no-ops after normal envelope sequencing.
- Reordered Journal input, exact duplicate Events, cancellation, concurrent
  rebuild serialization, Journal failure, conflict/gap/version checks, and
  prior-Snapshot preservation remain unchanged.
- `emptySnapshot` always returns five nonnil maps.
- `Snapshot.clone` deep-copies Team maps, Agent maps, and every dormant slice.
- Equal relevant facts are deterministic; conflicting repeated Team or Agent
  creates fail closed.

## Acceptance criteria

1. A valid S2-W15 two-fact batch rebuilds exactly one Team and one Main Agent.
2. Main-only, one-dormant, and two-dormant Teams preserve exact dormant
   metadata but create only one Agent read-model record.
3. Project/reusable same-ID shadow and project-Team/reusable-Agent fallback
   preserve exact independent Team and Agent scopes/versions.
4. Journal insertion and input order do not affect the final Snapshot.
5. Rebuilding from the real SQLite Journal after a new Projection instance
   yields the same Snapshot.
6. Malformed, missing, extra, stale, invalid-scope, invalid-digest,
   invalid-count, invalid-dormant, invalid-binding, invalid-envelope, orphan,
   missing-Main, duplicate-Main, and cross-link-mismatch facts fail closed.
7. Failed, canceled, concurrent, or closed-Journal rebuild never partially
   swaps Team/Agent maps or existing maps.
8. Caller mutation of returned maps, Team/Agent values, or dormant slices
   cannot change the stored Snapshot.
9. Existing Slice 1 and S2-W1 through S2-W15 behavior remains green.
10. No Event append, direct projection-table write, migration/schema change,
    Team Draft mutation, new Team/Agent resource, dormant AgentInstance,
    WorkItem/Run/Evidence/grant, capacity reservation, workspace, process/model/
    runtime execution, Bridge, claim, lease, credential, daemon/CLI/UI, network/
    filesystem/environment access, production goroutine, external action, or
    Slice 3 behavior is introduced.

## Mandatory RED tests

The Developer updates `internal/projection/projection_test.go` before production.
RED must fail on missing frozen S2-W16 fields/types/behavior only.

Required groups named `TestRebuildSavedTeamInstanceFacts...`:

1. real SQLite replay of exact valid two-fact batches and new Projection
   rebuild equivalence;
2. main-only/one/two dormant cardinality and exact read-model fields;
3. project/reusable shadow and reusable-Agent fallback;
4. input/insertion reorder plus agent-stream-before-team link validation;
5. Team payload/envelope/scope/digest/count/dormant failure matrix;
6. Agent payload/envelope/scope/digest/binding failure matrix;
7. orphan/missing/duplicate Main and cross-link mismatch matrix;
8. failed/canceled/concurrent/closed-Journal prior-Snapshot preservation;
9. map/value/dormant-slice mutation isolation; and
10. existing behavior plus static import/trust-boundary assertion.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/projection -run 'TestRebuildSavedTeamInstanceFacts'
  -count=1`
- Package full: `go test ./internal/projection -count=1`
- Repeated focused race:
  `go test -race ./internal/projection -run
  'TestRebuildSavedTeamInstanceFacts' -count=50`
- Impact: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/projection/projection.go
  internal/projection/projection_test.go` and `git diff --check`
- Import boundary: production remains standard library plus accepted
  `internal/journal` only.

## Explicit exclusions

No changes outside owned files. No Event append, projection table, direct SQL
write, migration/schema change, Team Draft mutation, new Team/Agent resource,
dormant AgentInstance, WorkItem/Run/Evidence/grant, capacity reservation,
workspace, process/model/runtime execution, Bridge, claim generation, lease,
credential, network/filesystem/environment access, production goroutine,
daemon/CLI/UI, external action, or Slice 3 behavior.

VERDICT: CONTRACT_FROZEN
