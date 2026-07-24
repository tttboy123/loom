# S2-W13 Frozen WorkItem Contract

- ID: `S2-W13`
- Title: Saved-Team Instance Record Set Candidate
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W8, S2-W9, S2-W11, S2-W12, and local S2-W12
  commit `e8ddc82`
- Corresponds to: `PRODUCT-PLAN.md §4.1-§4.3, §4.6`,
  `TECH-PLAN.md §3.2, §4, §5, §14 Slice 2, §15.2, §15.5, §15.8`, ADR-0001,
  and ADR-0003
- Frozen branch/head: `codex/loom-platform-slice2` at `e8ddc82`

## Owned files

- `internal/teams/saved_team_instances.go`
- `internal/teams/saved_team_instances_test.go`
- `.loom-evidence/phase1-slice2/S2-W13/deliverable.md`

Accepted Slice 1 and S2-W1 through S2-W12 product/test files remain unchanged.
Any ownership amendment requires a recorded Controller amendment and fresh
contract Reviewer PASS.

## Objective

Add a pure immutable resource-record set Candidate for the exact direct
saved-Team plan accepted by S2-W12:

1. revalidate the S2-W12 plan from the complete current routing, Team, Agent,
   RuntimeProfile, discovery, selection, and binding sources;
2. accept caller-supplied stable TeamInstance/Main AgentInstance identities,
   WorkRequest identity, and creation timestamp without allocating them;
3. resolve and freeze the exact current Main AgentDefinition version and scope
   selected for the saved Team;
4. produce one TeamInstance record and exactly one Main AgentInstance record;
5. preserve saved SubAgent members only as dormant records without
   AgentInstance IDs or active state; and
6. return deterministic immutable data with zero output on failure.

This WorkItem closes the domain-record shape required before a later atomic
state writer. It does not insert a row, append an Event, allocate an identity,
create a WorkItem/Run/grant/workspace/process, reserve capacity, or execute.

## Frozen input

```go
type SavedTeamInstanceIdentityInput struct {
    WorkRequestID      string
    TeamInstanceID     string
    MainAgentInstanceID string
    CreatedAt          int64
}

BuildSavedTeamInstanceRecordSet(
    plan SavedTeamInstantiationPlanCandidate,
    intent mode.Intent,
    context agents.ScopeIdentity,
    catalog TeamResolutionCatalogInput,
    binding SavedTeamRuntimeBindingCandidate,
    discovery runtime.RuntimeDiscoverySnapshot,
    selections []SavedTeamRuntimeSelection,
    identity SavedTeamInstanceIdentityInput,
) (SavedTeamInstanceRecordSetCandidate, error)
```

Construction:

1. calls accepted S2-W12 `ValidateSavedTeamInstantiationPlan` with the exact
   current sources and requires `Valid=true`;
2. requires non-empty WorkRequest, TeamInstance, and Main AgentInstance IDs and
   `CreatedAt > 0`;
3. resolves the Main AgentDefinition from the exact plan Team scope identity,
   requires its ID to equal the S2-W12 Main seed and S2-W11 Main binding, and
   freezes its current version/scope/scope identity;
4. requires the exact accepted Main RuntimeProfile/RuntimeInstance binding
   already revalidated by S2-W12;
5. copies and normalizes zero, one, or two dormant SubAgent records; and
6. computes the record-set digest only after all validation succeeds.

Existing S1/S2-W8/S2-W9/S2-W11/S2-W12 typed errors propagate without being
hidden. New typed errors distinguish invalid identity input, invalid record-set
shape, Main definition/source mismatch, dormant-state mismatch, record-source
mismatch, and record-set digest mismatch.

## Frozen result

`SavedTeamInstanceRecord` contains:

- exact supplied TeamInstance ID;
- exact supplied WorkRequest ID;
- source kind `saved_team`;
- exact TeamDefinition ID/version/scope/scope identity/digest;
- exact S2-W12 source plan digest;
- state `created`;
- exact supplied creation timestamp; and
- no Draft source.

`SavedTeamMainAgentInstanceRecord` contains:

- exact supplied Main AgentInstance ID;
- exact supplied TeamInstance ID;
- exact Main AgentDefinition ID/version/scope/scope identity;
- exact RuntimeProfile ID;
- exact RuntimeInstance ID;
- `IsMain=true`;
- state `created`; and
- exact accepted S2-W1 Runtime binding Candidate.

`SavedTeamDormantSubAgentRecord` contains:

- `Dormant=true`;
- exact AgentDefinition ID;
- exact RuntimeProfile ID;
- exact RuntimeInstance ID; and
- no AgentInstance ID, WorkItem, Run, grant, workspace, process, or active
  state.

`SavedTeamInstanceRecordSetCandidate` contains:

- `Ready=true`;
- exact Team record;
- exact Main Agent record;
- zero, one, or two normalized dormant SubAgent records;
- `TeamInstanceCount=1`;
- `AgentInstanceCount=1`;
- `ActiveSubAgentCount=0`;
- `WorkItemCount=0`;
- exact source plan digest; and
- deterministic lowercase SHA-256 record-set digest.

Dormant records normalize by AgentDefinition ID, RuntimeProfile ID, then
RuntimeInstance ID. The digest covers every semantic field. Accessors deep-copy
mutable slices.

The state `created` means only that these are the exact records a later atomic
writer may commit. It is not `running`, does not imply an Agent process exists,
and is not execution authority.

## Frozen validation

An exported pure validation function accepts the current record set plus the
same source and identity inputs, re-runs S2-W12 validation and Main definition
resolution, rebuilds the expected record set, compares every semantic field and
digest, and returns zero output on failure.

The validation Candidate contains only `Valid=true`, TeamInstance ID,
Main AgentInstance ID, TeamDefinition ID/digest, source plan digest,
record-set digest, and counts `(1,1,0,0)`. It is not persistence or execution
authority.

## Acceptance criteria

1. Valid project and reusable saved-Team plans produce one exact Team record and
   one exact Main Agent record.
2. Same-ID project/reusable AgentDefinition shadowing resolves in the exact Team
   scope, including the reusable-default path from project context.
3. Main-only, Main+one-SubAgent, and Main+two-SubAgent Teams retain zero/one/two
   dormant records but always create exactly one AgentInstance record.
4. Main AgentDefinition ID/version/scope/scope identity and Runtime binding are
   exact current catalog facts.
5. Empty identities or non-positive creation time fail closed with zero output.
6. Zero, tampered, stale, route-changed, catalog-changed, discovery-changed,
   selection-changed, binding-changed, or plan-changed sources fail closed.
7. Dormant SubAgents never receive AgentInstance IDs, active state, WorkItems,
   Runs, grants, workspaces, or processes.
8. Input/source/accessor mutation does not alter stored records or digest.
9. Equal semantics produce equal digest; changing any Team, Main Agent,
   dormant member, identity, timestamp, state, count, source plan digest, or
   Runtime binding field changes it.
10. Validation rejects zero, tampered, source-mismatched, dormant-active,
    count-invalid, state-invalid, and digest-invalid record sets with zero
    output.
11. Existing Slice 1 and S2-W1 through S2-W12 behavior remains green.
12. No identity allocation, Team Draft mutation, SQLite/Event write,
    transaction, persistence, WorkItem/Run/Evidence/grant creation, capacity
    reservation, workspace, process/model/runtime execution, Bridge, claim,
    lease, credential, daemon/CLI/UI, network, filesystem, environment,
    goroutine, external action, or Slice 3 behavior is introduced.

## Mandatory RED tests

The Developer adds `internal/teams/saved_team_instances_test.go` before
production. RED must fail on missing frozen S2-W13 symbols only.

Required groups:

1. project and same-ID-shadowed reusable saved-Team success;
2. Main-only/one/two dormant-member cardinality and exact record fields;
3. exact AgentDefinition version/scope and Runtime binding;
4. identity/timestamp and plan/source failure matrix with zero output;
5. source reorder determinism and every-field digest sensitivity;
6. input/source/accessor deep-copy isolation;
7. zero/tampered/source/dormant/count/state/digest validation failures with zero
   output; and
8. static import/trust-boundary assertion.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/teams -run
  'TestBuildSavedTeamInstanceRecordSet|TestValidateSavedTeamInstanceRecordSet'
  -count=1`
- Package full: `go test ./internal/teams -count=1`
- Repeated focused race:
  `go test -race ./internal/teams -run
  'TestBuildSavedTeamInstanceRecordSet|TestValidateSavedTeamInstanceRecordSet'
  -count=50`
- Impact: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/teams/saved_team_instances.go
  internal/teams/saved_team_instances_test.go` and `git diff --check`
- Import boundary: standard library plus accepted `internal/agents`,
  `internal/mode`, and `internal/runtime` contracts only.

## Explicit exclusions

No changes to accepted Slice 1 or S2-W1 through S2-W12 product/test files. No
identity allocation, Team Draft creation/mutation, state writer, Event/SQLite
write, transaction, persistence, WorkItem/Run/Evidence/grant creation, dormant
SubAgent activation, capacity reservation, workspace, process/model/runtime
execution, Bridge, claim generation, lease, credential, network/filesystem/
environment access, goroutine, daemon/CLI/UI, external action, or Slice 3
behavior.

VERDICT: CONTRACT_FROZEN
