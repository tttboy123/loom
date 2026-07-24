# S2-W8 Frozen WorkItem Contract

- ID: `S2-W8`
- Title: Immutable Saved TeamDefinition Core
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W1 commit `954416a` and S2-W7 commit `ab88c5c`
- Corresponds to: `PRODUCT-PLAN.md §3.2, §4.1-§4.3, §13 Phase 1`,
  `TECH-PLAN.md §3.2, §4, §14 Slice 2, §15.2, §15.5-§15.8`, and ADR-0003
- Frozen branch/head: `codex/loom-platform-slice2` at `ab88c5c`

## Owned files

- `internal/teams/team_definition.go`
- `internal/teams/team_definition_test.go`
- `.loom-evidence/phase1-slice2/S2-W8/deliverable.md`

Accepted S2-W1 through S2-W7 files remain unchanged. Any ownership amendment
requires a recorded Controller amendment and fresh contract Reviewer PASS.

## Objective

Add a pure immutable saved `TeamDefinition` domain contract that:

1. contains exactly one Main role and zero, one, or two SubAgent roles for the
   Phase 1 local capacity boundary;
2. references existing active S2-W1 AgentDefinitions and valid S2-W1
   RuntimeProfiles by stable ID without copying device state into the saved
   definition;
3. preserves Main coordination-only semantics and prevents the same Agent
   definition from occupying more than one role;
4. supports project and reusable TeamDefinition scopes with project precedence,
   exact scope identity, latest-version resolution, and archived exclusion;
5. produces deterministic immutable digests and copied accessors; and
6. returns a pure load Candidate for a selected complete TeamDefinition without
   creating a TeamInstance or Team Draft.

This WorkItem defines and resolves saved complete team structure only. S2-W9
will compose it with the explicit Mode Router and default-Main/Draft routing
rules. Later WorkItems will bind live RuntimeInstances and create resources.

## Frozen model

`TeamDefinitionScope` has exactly:

- `project`; and
- `reusable`.

`TeamDefinitionStatus` has exactly:

- `active`; and
- `archived`.

`TeamDefinitionRoleKind` has exactly:

- `main`; and
- `subagent`.

`TeamDefinitionRole` contains:

- role kind;
- stable AgentDefinition ID;
- stable RuntimeProfile ID; and
- non-empty responsibility.

`TeamDefinitionInput` contains:

- non-empty stable TeamDefinition ID;
- positive version;
- scope and exact S2-W1 scope identity;
- non-empty name;
- status; and
- roles.

`TeamDefinition` contains private copied normalized input plus a lowercase
SHA-256 digest. Roles normalize Main first and SubAgents by AgentDefinition ID,
then RuntimeProfile ID. Input order does not affect semantics or digest.

`TeamDefinitionLoadCandidate` contains:

- `Found=true`;
- exact TeamDefinition ID/version/scope/scope identity;
- exactly one Main AgentDefinition ID;
- normalized SubAgentDefinition IDs;
- normalized copied roles; and
- definition digest.

It is a resolution Candidate, not a TeamInstance, Draft, allocation, or
execution authority.

## Frozen constructors and resolution

```go
BuildTeamDefinition(
    input TeamDefinitionInput,
    definitions []agents.AgentDefinition,
    profiles []runtime.RuntimeProfile,
) (TeamDefinition, error)

ResolveTeamDefinition(
    definitions []TeamDefinition,
    id string,
    context agents.ScopeIdentity,
) (TeamDefinitionLoadCandidate, error)
```

Construction:

1. validates all S2-W1 AgentDefinitions and RuntimeProfiles through their
   accepted constructors/contracts;
2. rejects zero, malformed, archived, duplicate, or invented referenced
   AgentDefinition/Profile IDs;
3. resolves every referenced AgentDefinition under the TeamDefinition scope
   identity and requires the exact active result;
4. requires exactly one Main and at most two SubAgents;
5. rejects duplicate AgentDefinition IDs across all roles and duplicate role
   bindings;
6. requires every role responsibility to be non-empty;
7. stores RuntimeProfile identity/policy only, never RuntimeInstance/device
   state; and
8. copies all mutable inputs.

Resolution:

1. requires non-empty stable TeamDefinition ID and valid scope context;
2. validates every Candidate before selection;
3. selects matching active project scope first, then matching reusable scope;
4. within the winning scope selects the greatest version;
5. rejects duplicate winning ID/scope/version Candidates;
6. excludes archived Candidates;
7. is invariant to Candidate order; and
8. returns zero Candidate on every failure.

Typed errors must distinguish invalid TeamDefinition/input/role, invalid
definition/profile catalog, invented definition/profile reference, duplicate
role/reference/winning Candidate, not found, and digest mismatch.

An exported pure validation function revalidates a `TeamDefinition` against the
current definition/profile inputs and returns a zero validation Candidate on
failure.

## Acceptance criteria

1. Valid Main-only, Main+one-SubAgent, and Main+two-SubAgent saved definitions
   build and validate.
2. Zero Main, two Main, three SubAgents, duplicate AgentDefinition, duplicate
   binding, empty responsibility, invalid scope/status/version/name/identity,
   archived/invented definition, and invalid/invented profile fail closed.
3. RuntimeProfile changes do not require copying or rewriting AgentDefinition;
   each role binds the selected valid Profile ID independently.
4. No RuntimeInstance ID, online status, discovered model list, capacity,
   heartbeat, device identity, credential, or environment value enters the
   saved TeamDefinition.
5. Resolution proves project-over-reusable precedence, exact scope identity,
   latest version, archived exclusion, deterministic reorder, duplicate winner,
   invalid Candidate, and not-found behavior.
6. Load and validation failures return zero Candidates.
7. Input/accessor mutation does not alter roles, scope identity, or digest.
8. Digests are deterministic for semantic reorder and change for team identity,
   version, scope, scope identity, name, status, Agent/Profile binding, role
   kind, or responsibility changes.
9. The load Candidate preserves exactly one Main and normalized SubAgents but
   does not create or imply a TeamInstance, AgentInstance, WorkItem, or Run.
10. Existing Slice 1 and S2-W1 through S2-W7 behavior remains green.
11. No Team Draft, terminal decision, default Main selection, mode routing,
    persistence, Event/SQLite write, allocation, workspace, process, model call,
    Runtime execution, Bridge, Run, AgentGrant, credential, daemon/CLI/UI,
    network, filesystem, environment, goroutine, or activation is introduced.

## Mandatory RED tests

The Developer adds `internal/teams/team_definition_test.go` before production.
RED must fail on missing frozen S2-W8 symbols only.

Required groups:

1. valid zero/one/two-SubAgent definitions, normalized roles, exact
   AgentDefinition/Profile binding, and independent Profile switching;
2. every cardinality, identity, catalog, reference, status, responsibility, and
   duplicate failure with zero output;
3. project/reusable precedence, exact identity, latest version, archive,
   duplicate winner, invalid Candidate, reorder, and not-found resolution;
4. validation success plus zero/tampered/current-catalog mismatch rejection;
5. deterministic digest equality and field-by-field semantic sensitivity;
6. deep mutation isolation; and
7. static import/trust-boundary assertion.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/teams -run
  'TestTeamDefinition|TestResolveTeamDefinition|TestValidateTeamDefinition'
  -count=1`
- Package full:
  `go test ./internal/teams -count=1`
- Repeated focused race:
  `go test -race ./internal/teams -run
  'TestTeamDefinition|TestResolveTeamDefinition|TestValidateTeamDefinition'
  -count=50`
- Impact:
  `go test ./... -count=1`
- Repository race:
  `go test -race ./... -count=1`
- Static:
  `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/teams/team_definition.go
  internal/teams/team_definition_test.go` and `git diff --check`
- Import boundary: standard library plus accepted `internal/agents` and
  `internal/runtime` contracts only.

## Explicit exclusions

No changes to accepted S2-W1 through S2-W7 product/test files. No Team Draft or
decision mutation, default Main choice, mode routing, TeamInstance/
AgentInstance/WorkItem creation, RuntimeInstance binding, transaction,
Event/SQLite write, persistence, allocation, process/model/runtime execution,
Bridge, Run, AgentGrant, credential, network/filesystem/environment access,
goroutine, daemon/CLI/UI, external action, or Slice 3 behavior.

VERDICT: CONTRACT_FROZEN
