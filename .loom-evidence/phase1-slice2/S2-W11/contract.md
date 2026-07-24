# S2-W11 Frozen WorkItem Contract

- ID: `S2-W11`
- Title: Saved Team Runtime Binding Candidate
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W1, S2-W2, S2-W8, S2-W9, and local S2-W10
  commit `88eea03`
- Corresponds to: `PRODUCT-PLAN.md §4.1-§4.3, §4.6`,
  `TECH-PLAN.md §3.2, §4, §14 Slice 2, §15.2, §15.5, §15.7-§15.8`,
  ADR-0001, and ADR-0003
- Frozen branch/head: `codex/loom-platform-slice2` at `88eea03`

## Owned files

- `internal/teams/saved_team_binding.go`
- `internal/teams/saved_team_binding_test.go`
- `.loom-evidence/phase1-slice2/S2-W11/deliverable.md`

Accepted Slice 1 and S2-W1 through S2-W10 product/test files remain unchanged.
Any ownership amendment requires a recorded Controller amendment and fresh
contract Reviewer PASS.

## Objective

Add a pure immutable Runtime-binding Candidate for one exact resolved saved
S2-W8 TeamDefinition:

1. re-resolve the exact active TeamDefinition under project-over-reusable and
   latest-version rules;
2. require an explicit RuntimeInstance ID selection for every Main/SubAgent
   role and no extra selection;
3. resolve each role's exact accepted RuntimeProfile;
4. resolve each selected RuntimeInstance from one exact S2-W2 discovery
   snapshot;
5. call accepted S2-W1 `runtime.ValidateBinding` for every Profile/Instance
   pair;
6. require each selected discovery observation to advertise the exact
   RuntimeProfile model ID;
7. preserve exactly one Main and at most two SubAgent bindings; and
8. return a deterministic immutable Candidate with zero output on failure.

This WorkItem closes only the live Runtime compatibility dependency for the
saved-Team direct path. It does not allocate or create TeamInstance,
AgentInstance, WorkItem, Run, grant, process, workspace, or persistent state.

## Frozen input

```go
type SavedTeamRuntimeSelection struct {
    AgentDefinitionID string
    RuntimeInstanceID string
}

BuildSavedTeamRuntimeBinding(
    teamDefinitions []TeamDefinition,
    teamID string,
    context agents.ScopeIdentity,
    agentDefinitions []agents.AgentDefinition,
    profiles []runtime.RuntimeProfile,
    discovery runtime.RuntimeDiscoverySnapshot,
    selections []SavedTeamRuntimeSelection,
) (SavedTeamRuntimeBindingCandidate, error)
```

Input rules:

1. team ID is non-empty and context is a valid project scope;
2. the exact selected Team is re-resolved through S2-W8;
3. the selected definition digest is preserved in the result;
4. AgentDefinition and RuntimeProfile catalogs revalidate through accepted
   S2-W1/S2-W8 paths;
5. discovery digest is non-empty and every observation is revalidated;
6. selections are non-empty, copied, unique by AgentDefinition ID, and contain
   no empty field;
7. selections cover exactly every Team role once and contain no unselected or
   invented AgentDefinition;
8. each selected RuntimeInstance ID resolves exactly once from the current
   discovery snapshot;
9. each selected observation advertises the role RuntimeProfile's exact model
   ID in addition to passing `runtime.ValidateBinding`; and
10. catalog/input order does not affect the result.

Multiple Team roles may share one RuntimeInstance only when that instance's
positive advertised capacity is at least the number of selected bindings. No
selection or Candidate reserves or decrements capacity.

## Frozen result

`SavedTeamRuntimeRoleBinding` contains:

- exact S2-W8 role kind;
- exact AgentDefinition ID;
- exact RuntimeProfile ID;
- exact RuntimeInstance ID; and
- copied accepted S2-W1 `runtime.BindingCandidate`.

`SavedTeamRuntimeBindingCandidate` contains:

- `Ready=true`;
- exact TeamDefinition ID/version/scope/scope identity/digest;
- exact S2-W2 Runtime discovery digest;
- exactly one normalized Main binding;
- zero, one, or two normalized SubAgent bindings;
- total role count; and
- deterministic lowercase SHA-256 binding digest.

Bindings normalize Main first and SubAgents by AgentDefinition ID,
RuntimeProfile ID, then RuntimeInstance ID. The digest covers every listed
semantic field. Mutable input and returned slices/scope values are copied.

The Candidate contains no Runtime executable path, credential, environment
value, TeamInstance/AgentInstance/WorkItem/Run ID, allocation, reservation,
lease, process identity, Event sequence, or activation flag.

## Frozen validation

An exported pure validation function:

```go
ValidateSavedTeamRuntimeBinding(
    current SavedTeamRuntimeBindingCandidate,
    teamDefinitions []TeamDefinition,
    teamID string,
    context agents.ScopeIdentity,
    agentDefinitions []agents.AgentDefinition,
    profiles []runtime.RuntimeProfile,
    discovery runtime.RuntimeDiscoverySnapshot,
    selections []SavedTeamRuntimeSelection,
) (SavedTeamRuntimeBindingValidationCandidate, error)
```

Validation revalidates the Candidate, rebuilds the expected binding from the
exact current sources, compares every semantic field and digest, and returns
zero output on failure.

The validation Candidate contains only `Valid=true`, TeamDefinition ID/digest,
discovery digest, binding digest, Main AgentDefinition ID, and role count. It is
not allocation, persistence, or execution authority.

Typed errors must distinguish invalid input/Candidate, incomplete/extra/
duplicate role selection, missing or ambiguous RuntimeInstance source,
unavailable Profile model, insufficient shared capacity, source mismatch, and
binding-digest mismatch. Accepted S2-W1, S2-W2, and S2-W8
validation/binding errors propagate without being hidden.

## Acceptance criteria

1. Valid Main-only, Main+one-SubAgent, and Main+two-SubAgent saved Teams bind to
   exact compatible online RuntimeInstances.
2. Every role preserves its exact TeamDefinition Agent/Profile binding and
   explicit RuntimeInstance selection with no widening or invention.
3. Offline, incompatible, disabled, adapter-mismatched, capability-missing,
   model-missing, invalid, missing, or duplicate RuntimeInstance sources fail
   closed.
4. Missing, duplicate, extra, malformed, invented, or wrong-role selections
   fail closed.
5. Shared RuntimeInstance capacity equal to binding count succeeds; capacity
   below count fails; no capacity is mutated or reserved.
6. Team project/reusable precedence, exact scope, latest version, archived
   exclusion, and selected definition digest remain S2-W8 authoritative.
7. Catalog/discovery/selection reorder does not change the Candidate or digest.
8. Input/source/accessor mutation does not alter stored binding state or digest.
9. Equal semantics produce equal digest; changing Team identity/version/scope/
   digest, discovery digest, role kind, Agent/Profile/Runtime binding, or role
   count changes it.
10. Validation rejects zero, tampered, stale-source, selection-mismatched,
    discovery-mismatched, capacity-invalid, and digest-invalid Candidates with
    zero output.
11. Existing Slice 1 and S2-W1 through S2-W10 behavior remains green.
12. No Team Draft, decision, saved-Team mutation, default selection, Mode
    routing, requested budget/concurrency, task planning, resource ID
    allocation, TeamInstance/AgentInstance/WorkItem/Event/SQLite write,
    transaction, persistence, reservation, workspace, process, model call,
    Runtime execution, Bridge, Run, AgentGrant, credential, daemon/CLI/UI,
    network, filesystem, environment, goroutine, external action, or Slice 3
    behavior is introduced.

## Mandatory RED tests

The Developer adds `internal/teams/saved_team_binding_test.go` before
production. RED must fail on missing frozen S2-W11 symbols only.

Required groups:

1. valid zero/one/two-SubAgent binding, exact role/Profile/Runtime mapping,
   normalization, source digests, and validation Candidate;
2. Team/catalog/discovery/Profile/Runtime/selection failure classes with zero
   output and propagated typed errors;
3. exact shared-capacity boundary and proof that no capacity is mutated;
4. project/reusable/latest/archive resolution and source-digest binding;
5. reorder determinism and every-field digest sensitivity;
6. input/source/accessor deep-copy isolation;
7. zero/tampered/source/selection/discovery/capacity/digest validation failures
   with zero output; and
8. static import/trust-boundary assertion.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/teams -run
  'TestBuildSavedTeamRuntimeBinding|TestValidateSavedTeamRuntimeBinding'
  -count=1`
- Package full:
  `go test ./internal/teams -count=1`
- Repeated focused race:
  `go test -race ./internal/teams -run
  'TestBuildSavedTeamRuntimeBinding|TestValidateSavedTeamRuntimeBinding'
  -count=50`
- Impact: `go test ./... -count=1`
- Repository race: `go test -race ./... -count=1`
- Static: `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/teams/saved_team_binding.go
  internal/teams/saved_team_binding_test.go` and `git diff --check`
- Import boundary: standard library plus accepted `internal/agents` and
  `internal/runtime` contracts only.

## Explicit exclusions

No changes to accepted Slice 1 or S2-W1 through S2-W10 product/test files. No
Draft/decision mutation, saved-Team mutation, default routing, direct-start
task plan, resource ID allocation, TeamInstance/AgentInstance/WorkItem
creation, capacity reservation, transaction, Event/SQLite write, persistence,
workspace, process/model/runtime execution, Bridge, Run, AgentGrant, claim
generation, lease, credential, network/filesystem/environment access,
goroutine, daemon/CLI/UI, external action, or Slice 3 behavior.

VERDICT: CONTRACT_FROZEN
