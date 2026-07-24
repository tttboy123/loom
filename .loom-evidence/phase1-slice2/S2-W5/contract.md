# S2-W5 Frozen WorkItem Contract

- ID: `S2-W5`
- Title: Structured Team Draft Content Core
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W1 commit `954416a`, S2-W3 commit `e196107`,
  and S2-W4 commit `0a98851`
- Corresponds to: `PRODUCT-PLAN.md §4.3-§4.6`, `TECH-PLAN.md §1.1.2-§1.1.5,
  §3.3, §14 Slice 2, and §15.4-§15.10`
- Frozen branch/head: `codex/loom-platform-slice2` at `0a98851`

## Owned files

- `internal/teams/draft_content.go`
- `internal/teams/draft_content_test.go`
- `.loom-evidence/phase1-slice2/S2-W5/deliverable.md`

The Developer owns only these files. S2-W4 Draft revision files remain
unchanged. Any ownership amendment requires a recorded Controller amendment and
fresh contract Reviewer PASS.

## Why this WorkItem precedes acceptance

The accepted S2-W4 core stores catalog references, revision state, one question,
and answer metadata. It intentionally does not yet represent all structured
content required by `TECH-PLAN.md §3.3`: per-Agent RuntimeProfile and compatible
RuntimeInstance suggestions, a first task graph, acceptance conditions, or the
customer-rule summary.

Accepting or instantiating that partial core would be premature. S2-W5 therefore
creates a separate immutable structured-content Candidate. A later WorkItem must
bind its digest to an S2-W4 revision and re-check it before any terminal Draft
transition or resource creation.

## Objective

Create a pure immutable `TeamDraftContentSnapshot` that:

1. is bound to one accepted S2-W3 catalog digest and complete normalized
   `TeamDraftReferences`;
2. assigns every selected Main/SubAgent exactly one validated RuntimeProfile and
   compatible online RuntimeInstance/model pair;
3. represents a deterministic first task-plan Candidate whose delivery owners
   are SubAgents, whose dependencies are acyclic, and whose acceptance criteria
   are explicit;
4. records a non-empty customer-rule summary plus bounded approval markers;
5. records bounded explicit capability gaps without inventing catalog IDs;
6. returns an acceptance-readiness Candidate only when the content is
   structurally complete and contains no capability gap; and
7. remains Candidate data with no state transition, allocation, persistence, or
   execution authority.

## Frozen input model

`TeamDraftContentLimits` contains strictly positive:

- `MaxTasks`;
- `MaxDependenciesPerTask`;
- `MaxAcceptanceCriteriaPerTask`;
- `MaxApprovalMarkers`; and
- `MaxCapabilityGaps`.

The trusted caller supplies these ceilings. Exactly `max` is accepted and
`max + 1` is rejected without truncation. The limits are stored in the snapshot
and participate in its digest.

`TeamDraftRoleSelection` contains:

- `AgentDefinitionID`;
- one complete S2-W1 `runtime.RuntimeProfile`;
- `RuntimeInstanceID`;
- Agent-scoped `SkillIDs`;
- Agent-scoped `MemberIDs`; and
- Agent-scoped `PermissionIDs`.

Main status is derived only from
`TeamDraftReferences.MainAgentDefinitionID`; no caller-provided `is_main` flag
exists. Every Main/SubAgent selected by the references appears exactly once.
No extra Agent may appear. Phase 1 requires at least one and permits at most two
SubAgents. Main-only content fails with `ErrMissingTeamDraftRole`; it is never a
valid or acceptance-ready snapshot.

Each role's RuntimeProfile:

- passes accepted S2-W1 shape validation through `runtime.ValidateBinding`;
- binds the catalog's exact online RuntimeInstance;
- has `ModelID` equal to a catalog-valid
  `(RuntimeInstanceID, ModelID)` pair selected in the references;
- has an adapter and required capabilities compatible with that instance; and
- is copied deeply, including required-capability slices and optional budget.

The unique RuntimeInstance IDs and Runtime/model pairs used by all roles must
equal, not merely be a subset of, those in the complete references. Sharing a
compatible RuntimeInstance or RuntimeProfile across roles is allowed and does
not allocate capacity.

Role Skill, member, and permission sets must be normalized unique subsets of
the corresponding complete reference sets. They do not grant tools or access.

`TeamDraftTaskCandidate` contains:

- non-empty stable task ID;
- one delivery-owner AgentDefinition ID;
- zero or more dependency task IDs; and
- one or more non-empty acceptance criteria.

At least one task is required. Task IDs are unique. Dependencies are unique,
refer only to tasks in the same content, cannot self-reference, and form a DAG.
A Main Agent cannot own a delivery task. Every selected SubAgent owns at least
one task. Tasks, dependency sets, and acceptance-criteria sets are normalized
deterministically.

`TeamDraftApprovalMarker` contains a non-empty stable marker ID and non-empty
reason. It is a local Candidate annotation only; it is not a Rule match,
ApprovalRequest, Grant, or user approval.

`TeamDraftCapabilityGap` contains a non-empty capability description and
non-empty reason. Gaps are explicit free-text deficiencies, not substitute IDs.
They are normalized and deduplicated. Any gap makes the content structurally
valid but `AcceptanceReady=false`.

`TeamDraftContentInput` contains:

- complete `TeamDraftReferences`;
- all role selections;
- all first-task candidates;
- a non-empty customer-rule summary;
- approval markers;
- capability gaps; and
- the trusted content limits.

## Construction and snapshot

`BuildTeamDraftContent(catalog, input)`:

1. rejects a zero catalog;
2. reuses S2-W3 complete-reference membership and ceiling validation;
3. validates and normalizes roles, Runtime bindings, tasks, markers, gaps, and
   limits;
4. computes a deterministic lowercase SHA-256 digest over the canonical
   semantic snapshot; and
5. returns a private immutable snapshot or a zero snapshot with typed error.

`TeamDraftContentSnapshot` copied accessors expose:

- content digest and catalog digest;
- normalized complete references;
- normalized roles;
- normalized tasks;
- customer-rule summary;
- normalized approval markers;
- normalized capability gaps; and
- the frozen limits.

Reordering semantic sets does not change the digest. Changing any semantic
field, reference, RuntimeProfile binding, task edge/criterion, rule summary,
marker, gap, or limit changes the digest.

`ValidateTeamDraftContent(snapshot, catalog)` re-checks:

- snapshot shape and digest integrity;
- exact catalog binding;
- complete S2-W3 references;
- every role RuntimeProfile/RuntimeInstance/model binding;
- task graph, role ownership, markers, gaps, and limits.

Success returns a pure `TeamDraftContentValidationCandidate` containing:

- `Valid=true`;
- `AcceptanceReady=true` only for the already-valid one-Main,
  one-or-two-SubAgent, non-empty-task content when no capability gap exists;
- content and catalog digests;
- Main AgentDefinition ID;
- role count; and
- task count.

It does not attach content to a Draft revision or authorize acceptance.

## Failure and typed-error semantics

Typed errors inspectable with `errors.Is` cover:

- invalid content or limits;
- content limit exceeded;
- invalid, duplicate, missing, or extra role;
- RuntimeProfile/model mismatch;
- invalid or duplicate task;
- unknown/self task dependency;
- cyclic task graph;
- Main Agent delivery assignment;
- SubAgent without a task;
- invalid or duplicate approval marker;
- invalid or duplicate capability gap;
- content digest mismatch; and
- wrapped S2-W3 catalog-reference and S2-W1 Runtime binding errors.

Every failure returns a zero snapshot or zero validation Candidate. Inputs and
existing snapshots remain unchanged.

## Acceptance boundary

1. The two product files import only standard library, accepted S2-W1
   `internal/runtime`, and accepted package-local S2-W3/S2-W4 types/functions.
2. Snapshot fields are private; all slice and RuntimeProfile budget data is
   deep copied on input and access.
3. Selected roles exactly cover one Main plus one or two referenced SubAgents,
   with no duplicate or extra Agent; main-only content fails closed.
4. Every role binding passes S2-W1 compatibility plus exact S2-W3 Runtime/model
   membership; no capacity is reserved.
5. RuntimeInstance and Runtime/model sets equal the complete reference sets.
6. At least one task exists; Main owns no delivery task; every SubAgent owns at
   least one; the graph is a bounded DAG with explicit criteria.
7. Rule summary, markers, and gaps are data only; no approval or authority is
   inferred.
8. Only gap-free valid content with at least one SubAgent and task returns
   `AcceptanceReady=true`.
9. Digest, ordering, mutation isolation, and revalidation are deterministic.
10. Existing Slice 1 and S2-W1 through S2-W4 behavior remains green.
11. No Draft revision attachment/transition, acceptance/rejection/expiry,
    user-confirmation record, default Main policy, TeamDefinition load,
    TeamInstance/AgentInstance, WorkItem/Event/SQLite write, allocation,
    workspace, process, model call, Runtime execution, Bridge, Run, AgentGrant,
    credential, daemon/CLI/UI, network, filesystem, environment, goroutine, or
    activation is introduced.

## Mandatory RED tests

The Developer adds `internal/teams/draft_content_test.go` before production.
RED must fail on missing frozen S2-W5 symbols only.

Required groups:

1. valid immutable normalized content, exact role coverage, exact Runtime/model
   coverage, deterministic digest, and mutation isolation;
2. zero catalog, invalid limits, exact-max acceptance, and max-plus-one
   rejection for every ceiling;
3. invalid/duplicate/missing/extra roles, main-only content, more than two
   SubAgents, invalid RuntimeProfile, adapter/capability/model mismatch,
   invented references, and incomplete/extra Runtime coverage;
4. valid reordered DAG plus zero/invalid/duplicate tasks, unknown/self
   dependency, cycle, Main ownership, unassigned SubAgent, missing criteria, and
   per-task ceilings;
5. rule-summary, approval-marker, and capability-gap validation, normalization,
   bounds, mutation isolation, and gap-driven readiness;
6. deterministic digest equality under set reordering and inequality for each
   semantic class;
7. snapshot revalidation success plus zero, tampered, and catalog-mismatch
   rejection;
8. static import/trust-boundary assertion.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/teams -run
  'TestTeamDraftContent|TestValidateTeamDraftContent' -count=1`
- Package full:
  `go test ./internal/teams -count=1`
- Repeated focused race:
  `go test -race ./internal/teams -run
  'TestTeamDraftContent|TestValidateTeamDraftContent' -count=50`
- Impact:
  `go test ./... -count=1`
- Repository race:
  `go test -race ./... -count=1`
- Static analysis:
  `go vet ./...`
- Formatting, diff, and scope:
  `gofmt` changed Go files, `git diff --check`, exact owned-file audit, unchanged
  branch/head before authorized commit.

## Required evidence

- frozen contract digest;
- exact RED output and exit code;
- all deterministic check outputs;
- role/runtime/model coverage, DAG, limit, digest, mutation, readiness, and
  import-boundary reports;
- trust-boundary analysis;
- fresh independent implementation Reviewer verdict;
- deliverable ending with `VERDICT: PASS` only after all gates pass.

## Trust-boundary analysis

- Structured content is model/user-edited Candidate data, not Event Journal or
  execution authority.
- Runtime compatibility proves only shape, online status, adapter,
  capabilities, and catalog model membership at snapshot time; it does not
  reserve capacity, authenticate, or start a process.
- Task candidates are not WorkItems and cannot be claimed or executed.
- Acceptance criteria are text requirements, not proof that they passed.
- Approval markers and customer-rule summary do not match Rules, create
  ApprovalRequests, or grant permission.
- Capability gaps explicitly prevent acceptance readiness and cannot smuggle
  invented IDs.
- No raw credential, token, executable output, environment value, filesystem
  path, Provider response, or user identity enters the content.

## Governance and next gate

- No product write before fresh independent contract Reviewer `PASS`.
- Active continuous authorization permits mandatory RED after that PASS.
- One Developer owns the three frozen files; Controller owns checks/evidence.
- Developer ends at `ready_for_review`; fresh implementation Reviewer decides
  PASS or bounded repair.
- Three failed bounded product repairs require `HUMAN_REQUIRED`.
- After all checks and fresh implementation Reviewer PASS, exactly one scoped
  local S2-W5 atomic commit is authorized.
- Push, merge, rebase, reset, force, release, publish, credentials, `.env`, new
  dependency, external mutation, paid remote work, daemon/Runtime activation,
  and autonomous execution remain prohibited.
