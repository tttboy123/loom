# S2-W10 Frozen WorkItem Contract

- ID: `S2-W10`
- Title: Accepted Draft Instantiation Plan Candidate
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W3 through S2-W7 and local S2-W9 commit `c5e9eed`
- Corresponds to: `PRODUCT-PLAN.md §4.4-§4.6`, `TECH-PLAN.md §3.3,
  §4, §14 Slice 2, §15.3-§15.5, §15.8-§15.10`, ADR-0001, and ADR-0003
- Frozen branch/head: `codex/loom-platform-slice2` at `c5e9eed`

## Owned files

- `internal/teams/instantiation_plan.go`
- `internal/teams/instantiation_plan_test.go`
- `.loom-evidence/phase1-slice2/S2-W10/deliverable.md`

Accepted Slice 1 and S2-W1 through S2-W9 product/test files remain unchanged.
Any ownership amendment requires a recorded Controller amendment and fresh
contract Reviewer PASS.

## Objective

Add a pure immutable instantiation-plan Candidate for one exact accepted S2-W7
structured Team Draft:

1. only an exact valid `accepted` decision produced by explicit user
   `confirm_and_start` may produce a plan;
2. the plan preserves the exact decision, Draft, catalog, content, and binding
   digests;
3. the plan contains exactly one normalized Main role seed, one or two
   normalized SubAgent role seeds, and the accepted first-task DAG;
4. every role seed preserves its accepted AgentDefinition, RuntimeProfile,
   online RuntimeInstance, Skill, member, and permission selections;
5. the plan preserves the exact accepted requested budget and concurrency
   separately from their catalog ceilings;
6. Main remains coordination-only and every SubAgent owns at least one task;
7. every successful plan is deterministic, immutable, and digest-bound; and
8. every failure returns the zero Candidate.

This WorkItem answers only “what exact accepted resources would a later atomic
creator create?” It does not allocate a TeamInstance, AgentInstance, WorkItem,
Run, grant, process, workspace, or persistent identity and does not write any
state.

## Frozen model

`TeamInstantiationRoleKind` has exactly:

- `main`; and
- `subagent`.

`TeamInstantiationRoleSeed` contains:

- role kind;
- exact AgentDefinition ID;
- a copied accepted RuntimeProfile;
- exact online RuntimeInstance ID;
- normalized copied Skill IDs;
- normalized copied member IDs; and
- normalized copied permission IDs.

`TeamInstantiationWorkItemSeed` contains:

- exact accepted Draft task ID;
- owner AgentDefinition ID;
- normalized copied dependency task IDs; and
- normalized copied acceptance criteria.

`TeamInstantiationPlanCandidate` contains:

- `Ready=true`;
- source kind fixed to `accepted_draft`;
- exact Draft ID and source/terminal revisions;
- exact catalog, content, binding, and decision digests;
- exactly one copied Main role seed;
- one or two normalized copied SubAgent role seeds;
- the normalized copied first-task DAG;
- copied customer-rule summary and approval markers;
- exact non-negative accepted requested budget and exact positive requested
  concurrency;
- exact catalog budget ceiling and catalog concurrency ceiling, stored
  separately from the requested values;
- no capability gaps; and
- a deterministic lowercase SHA-256 plan digest.

The plan digest canonically covers every listed semantic field. Role order,
task order, dependency order, acceptance-criteria order, and annotation order
do not affect the digest after accepted-contract normalization. Mutable slices
and RuntimeProfile internals are copied on input, storage, and access.

The Candidate contains no TeamInstance ID, AgentInstance ID, WorkItem runtime
ID, Run ID, AgentGrant, claim generation, lease, workspace path, credential,
Event sequence, process identity, or activation flag.

## Frozen constructors and validation

```go
BuildAcceptedDraftInstantiationPlan(
    decided DecidedTeamDraft,
    catalog TeamDraftCatalogSnapshot,
) (TeamInstantiationPlanCandidate, error)

ValidateAcceptedDraftInstantiationPlan(
    current TeamInstantiationPlanCandidate,
    decided DecidedTeamDraft,
    catalog TeamDraftCatalogSnapshot,
) (TeamInstantiationPlanValidationCandidate, error)
```

Construction:

1. calls accepted S2-W7 `ValidateDecidedTeamDraft` against the exact current
   catalog;
2. requires terminal kind `accepted`; rejected, expired, zero, tampered, stale,
   or catalog-mismatched decisions fail closed;
3. revalidates the exact S2-W6 source binding and S2-W5 content through the
   accepted decision validation path;
4. requires gap-free acceptance-ready content, exactly one Main, one or two
   SubAgents, and the accepted bounded task DAG;
5. maps every accepted role to exactly one role seed and preserves its exact
   Profile/Runtime/Skill/member/permission selections;
6. maps every accepted task to one WorkItem seed without changing ownership,
   dependencies, or acceptance criteria;
7. requires Main to own no task and every SubAgent to own at least one task;
8. copies requested budget/concurrency from the exact accepted Draft
   references, requires budget to remain non-negative and concurrency to remain
   positive, and requires each not to exceed its corresponding current catalog
   ceiling;
9. carries the catalog budget/concurrency ceilings as distinct fields plus the
   accepted customer-rule and approval metadata without expanding them;
10. normalizes and deep-copies all mutable fields; and
11. computes the plan digest only after all validation succeeds.

Typed errors must distinguish an invalid plan, a non-accepted terminal
decision, role/task mapping inconsistency, and plan-digest mismatch. Existing
S2-W3 through S2-W7 typed validation errors propagate without being hidden.

Validation revalidates both the current plan and exact accepted decision against
the current catalog, rebuilds the expected Candidate, compares every semantic
field and digest, and returns zero output on failure.

`TeamInstantiationPlanValidationCandidate` contains only:

- `Valid=true`;
- exact Draft ID;
- exact decision digest;
- exact plan digest;
- Main AgentDefinition ID;
- role count; and
- task count.

It is evidence that the plan is internally valid, not resource-creation or
execution authority.

## Acceptance criteria

1. A valid exact accepted Draft with one or two SubAgents produces one Main
   seed, exact SubAgent seeds, exact task seeds, exact source digests, and
   `Ready=true`.
2. Rejected, expired, zero, tampered, stale, catalog-mismatched, ineligible, or
   otherwise invalid decisions fail closed with zero output.
3. Role seeds preserve exact accepted Agent/Profile/online RuntimeInstance/
   Skill/member/permission selections and never invent or widen a selection.
4. WorkItem seeds preserve the exact bounded DAG, owners, dependencies, and
   acceptance criteria.
5. Main owns no delivery task; every accepted SubAgent owns at least one task.
6. No capability gap may enter a ready plan.
7. Accepted requested budget/concurrency and catalog ceilings are copied
   exactly into distinct fields; requested budget remains non-negative,
   requested concurrency remains positive, both remain within their ceilings,
   and no plan field can increase either accepted value.
8. Input/source/accessor mutation cannot change stored plan state, source
   decision, RuntimeProfile internals, slices, annotations, or digest.
9. Equal semantics produce equal digests; changing any source digest, role
   binding/selection, task field, customer-rule summary, approval marker,
   requested budget/concurrency, budget ceiling, or concurrency ceiling changes
   the digest.
10. Validation rejects zero, tampered, source-mismatched, catalog-mismatched,
    requested-value-mismatched, ceiling-overflow, role/task-inconsistent, and
    digest-invalid plans with zero output.
11. Existing Slice 1 and S2-W1 through S2-W9 behavior remains green.
12. No saved Team direct-load conversion, TeamDefinition mutation, new Draft
    revision/decision, resource ID allocation, TeamInstance/AgentInstance/
    WorkItem/Event/SQLite write, transaction, persistence, workspace, process,
    model call, Runtime execution, Bridge, Run, AgentGrant, credential,
    daemon/CLI/UI, network, filesystem, environment, goroutine, external
    action, or Slice 3 behavior is introduced.

## Mandatory RED tests

The Developer adds `internal/teams/instantiation_plan_test.go` before
production. RED must fail on missing frozen S2-W10 symbols only.

Required groups:

1. exact accepted success for one and two SubAgents, normalized role/task
   seeds, source binding, distinct accepted requested budget/concurrency,
   catalog ceilings, annotations, and validation Candidate;
2. rejected/expired/zero/tampered/stale/catalog/content/reference/binding/
   decision failure propagation with zero output;
3. exact role Profile/Runtime/Skill/member/permission preservation and no
   widening;
4. exact task DAG/owner/dependency/acceptance preservation, Main
   coordination-only, and every-SubAgent-assigned proof;
5. zero requested-budget success, negative-budget failure propagated from
   S2-W3, positive requested concurrency, exact requested-value preservation,
   current-ceiling enforcement, source/plan mismatch rejection, and digest
   sensitivity;
6. deterministic semantic reorder and all remaining digest-field sensitivity;
7. input, source, returned accessor, RuntimeProfile, annotation, and slice
   deep-copy isolation;
8. zero/tampered/source-mismatched/catalog-mismatched/digest-invalid validation
   failures with zero output; and
9. static import/trust-boundary assertion.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/teams -run
  'TestBuildAcceptedDraftInstantiationPlan|TestValidateAcceptedDraftInstantiationPlan'
  -count=1`
- Package full:
  `go test ./internal/teams -count=1`
- Repeated focused race:
  `go test -race ./internal/teams -run
  'TestBuildAcceptedDraftInstantiationPlan|TestValidateAcceptedDraftInstantiationPlan'
  -count=50`
- Impact:
  `go test ./... -count=1`
- Repository race:
  `go test -race ./... -count=1`
- Static:
  `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/teams/instantiation_plan.go
  internal/teams/instantiation_plan_test.go` and `git diff --check`
- Import boundary: production may use standard library plus accepted
  package-local teams and `internal/runtime` contracts only.

## Explicit exclusions

No changes to accepted Slice 1 or S2-W1 through S2-W9 product/test files. No
saved-Team direct path, TeamDefinition build/mutation, Draft mutation/decision,
TeamInstance/AgentInstance/WorkItem creation, resource ID allocation,
RuntimeInstance binding beyond copied accepted IDs, transaction, Event/SQLite
write, persistence, workspace, process/model/runtime execution, Bridge, Run,
AgentGrant, claim generation, lease, credential, network/filesystem/environment
access, goroutine, daemon/CLI/UI, external action, or Slice 3 behavior.

VERDICT: CONTRACT_FROZEN
