# S2-W12 Frozen WorkItem Contract

- ID: `S2-W12`
- Title: Direct Saved-Team Instantiation Plan
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W8, S2-W9, S2-W11, and local S2-W11 commit
  `d5850f2`
- Corresponds to: `PRODUCT-PLAN.md §3.2, §4.1-§4.3, §4.6`,
  `TECH-PLAN.md §3.2-§3.3, §4, §14 Slice 2, §15.1-§15.2, §15.5,
  §15.8, §15.10`, ADR-0001, and ADR-0003
- Frozen branch/head: `codex/loom-platform-slice2` at `d5850f2`

## Owned files

- `internal/teams/saved_team_instantiation.go`
- `internal/teams/saved_team_instantiation_test.go`
- `.loom-evidence/phase1-slice2/S2-W12/deliverable.md`

Accepted Slice 1 and S2-W1 through S2-W11 product/test files remain unchanged.
Any ownership amendment requires a recorded Controller amendment and fresh
contract Reviewer PASS.

## Objective

Add a pure immutable direct-instantiation plan Candidate for an exact saved
Team selected through explicit Agent Mode:

1. re-run accepted S1/S2-W9 routing and require a `load_team` resolution;
2. revalidate the exact S2-W11 saved-Team Runtime binding against current
   Team/Agent/Profile/discovery/selection sources;
3. require resolution Team identity/digest to equal binding Team identity/
   digest;
4. plan one TeamInstance seed and exactly one Main AgentInstance seed;
5. preserve zero, one, or two saved SubAgent bindings as dormant candidates,
   not active AgentInstances;
6. create no Team Draft for the complete saved-Team path; and
7. return deterministic immutable data with zero output on failure.

This WorkItem defines what a later atomic creator may instantiate for the
direct saved-Team path. It does not allocate IDs, create resources, write state,
start a Runtime, or activate a SubAgent.

## Frozen input

```go
BuildSavedTeamInstantiationPlan(
    intent mode.Intent,
    context agents.ScopeIdentity,
    catalog TeamResolutionCatalogInput,
    binding SavedTeamRuntimeBindingCandidate,
    discovery runtime.RuntimeDiscoverySnapshot,
    selections []SavedTeamRuntimeSelection,
) (SavedTeamInstantiationPlanCandidate, error)
```

Construction:

1. calls accepted S2-W9 `ResolveAgentModeTeam(intent, context, catalog)`;
2. requires `Resolved=true`, kind `load_team`, `RequiresDraft=false`, and an
   exact saved Team load Candidate;
3. ordinary conversation, caller-forced mode, `direct_main`, `draft_seed`,
   malformed, not-found, or ambiguous routing fails closed;
4. calls accepted S2-W11 `ValidateSavedTeamRuntimeBinding` with the exact
   current TeamDefinition, AgentDefinition, RuntimeProfile, discovery, and
   selection sources;
5. requires exact Team ID/version/scope/scope identity/definition digest
   equality between resolution and binding;
6. requires exactly one Main binding and zero, one, or two SubAgent bindings;
7. copies all mutable input and Candidate fields; and
8. computes the plan digest only after all validation succeeds.

Existing S1, S2-W8, S2-W9, and S2-W11 typed errors propagate without being
hidden. New typed errors distinguish invalid plan input, non-saved-Team
resolution, resolution/binding mismatch, invalid dormant-member state,
plan-source mismatch, and plan-digest mismatch.

## Frozen result

`SavedTeamMainInstanceSeed` contains:

- exact Main AgentDefinition ID;
- exact RuntimeProfile ID;
- exact RuntimeInstance ID; and
- exact accepted S2-W1 Runtime binding Candidate.

`SavedTeamDormantSubAgentSeed` contains:

- `Dormant=true`;
- exact SubAgent AgentDefinition ID;
- exact RuntimeProfile ID;
- exact RuntimeInstance ID; and
- no AgentInstance ID, WorkItem, Run, grant, workspace, process, or active
  status.

`SavedTeamInstantiationPlanCandidate` contains:

- `Ready=true`;
- exact explicit trigger and target ID, excluding `intent.Text`;
- exact S2-W9 resolution digest;
- exact TeamDefinition ID/version/scope/scope identity/digest;
- exact S2-W11 Runtime discovery and binding digests;
- one copied Main seed;
- zero, one, or two normalized dormant SubAgent seeds;
- `CreateTeamInstance=true`;
- `CreateMainAgentInstance=true`;
- `CreateSubAgentInstances=false`;
- `RequiresDraft=false`;
- `WorkItemCount=0`; and
- deterministic lowercase SHA-256 plan digest.

Dormant SubAgents normalize by AgentDefinition ID, Profile ID, then
RuntimeInstance ID. The digest covers every listed semantic field. Candidate
and accessors deep-copy mutable slices and scope values.

The dormant representation is not an active Team alias: a SubAgent may become
active only through a later explicit WorkItem-bound contract that creates its
AgentInstance, Run, grant, and Evidence lifecycle. Until then it cannot be
reported as executing.

## Frozen validation

An exported pure validation function accepts the current plan plus the same
source inputs, re-runs routing and binding validation, rebuilds the expected
plan, compares every semantic field/digest, and returns zero output on failure.

The validation Candidate contains only `Valid=true`, TeamDefinition ID/digest,
resolution digest, binding digest, plan digest, Main AgentDefinition ID,
dormant SubAgent count, and WorkItem count zero. It is not resource-creation or
execution authority.

## Acceptance criteria

1. Explicit `select_team`, `use_agent` project/reusable default Team, and
   Team-only `assign` `load_team` paths can produce the exact direct plan.
2. Plain/unknown/empty trigger, caller-forced Agent mode, selected Main,
   selected non-Main, default-Main Draft seed, Agent-only assign, missing, and
   ambiguous routing fail closed.
3. Resolution and Runtime binding must match exact Team ID/version/scope/scope
   identity/definition digest.
4. Valid Main-only, Main+one-SubAgent, and Main+two-SubAgent Teams produce
   exactly one Main seed and zero/one/two dormant SubAgent seeds.
5. No Team Draft is produced for a complete saved Team.
6. No dormant SubAgent is marked active or assigned a WorkItem/Run/grant/
   workspace/process.
7. Changing only `intent.Text` does not change the plan or digest.
8. Catalog/team/discovery/selection reorder does not change the plan or digest.
9. Input/source/accessor mutation does not alter stored plan data or digest.
10. Equal semantics produce equal digest; changing trigger, target, resolution
    digest, Team identity/digest, discovery/binding digest, Main binding,
    dormant binding, create flags, Draft flag, or WorkItem count changes it.
11. Validation rejects zero, tampered, route-changed, source-mismatched,
    binding-mismatched, dormant-active, nonzero-WorkItem, and digest-invalid
    plans with zero output.
12. Existing Slice 1 and S2-W1 through S2-W11 behavior remains green.
13. No Draft creation/mutation, resource ID allocation,
    TeamInstance/AgentInstance/WorkItem/Event/SQLite write, transaction,
    persistence, capacity reservation, workspace, process, model call, Runtime
    execution, Bridge, Run, AgentGrant, claim generation, lease, credential,
    daemon/CLI/UI, network, filesystem, environment, goroutine, external
    action, or Slice 3 behavior is introduced.

## Mandatory RED tests

The Developer adds `internal/teams/saved_team_instantiation_test.go` before
production. RED must fail on missing frozen S2-W12 symbols only.

Required groups:

1. `select_team`, default saved-Team, and Team-only `assign` success;
2. every non-`load_team` Router/Resolver failure with zero output;
3. Main-only/one/two dormant SubAgent cardinality and exact binding;
4. resolution/binding Team identity and digest mismatch failures;
5. text exclusion, source reorder determinism, and every-field digest
   sensitivity;
6. input/source/accessor deep-copy isolation;
7. zero/tampered/route/source/binding/dormant/WorkItem/digest validation
   failures with zero output; and
8. static import/trust-boundary assertion.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/teams -run
  'TestBuildSavedTeamInstantiationPlan|TestValidateSavedTeamInstantiationPlan'
  -count=1`
- Package full: `go test ./internal/teams -count=1`
- Repeated focused race:
  `go test -race ./internal/teams -run
  'TestBuildSavedTeamInstantiationPlan|TestValidateSavedTeamInstantiationPlan'
  -count=50`
- Impact: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/teams/saved_team_instantiation.go
  internal/teams/saved_team_instantiation_test.go` and `git diff --check`
- Import boundary: standard library plus accepted `internal/agents`,
  `internal/mode`, and `internal/runtime` contracts only.

## Explicit exclusions

No changes to accepted Slice 1 or S2-W1 through S2-W11 product/test files. No
Draft creation/mutation, saved-Team or binding mutation, task generation,
resource ID allocation, TeamInstance/AgentInstance/WorkItem creation, dormant
SubAgent activation, capacity reservation, transaction, Event/SQLite write,
persistence, workspace, process/model/runtime execution, Bridge, Run,
AgentGrant, claim generation, lease, credential, network/filesystem/environment
access, goroutine, daemon/CLI/UI, external action, or Slice 3 behavior.

VERDICT: CONTRACT_FROZEN
