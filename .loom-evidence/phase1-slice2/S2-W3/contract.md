# S2-W3 Frozen WorkItem Contract

- ID: `S2-W3`
- Title: Bounded Team Draft Catalog Snapshot
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S2-W1 commit `954416a` and accepted S2-W2 commit
  `1170062`
- Corresponds to: `TECH-PLAN.md §1.1.11, §3.3, §4, §13.1,
  §14 Slice 2.6, and §15.9`
- Frozen branch/head: `codex/loom-platform-slice2` at `1170062`

## Owned files

- `internal/teams/catalog.go`
- `internal/teams/catalog_test.go`
- `.loom-evidence/phase1-slice2/S2-W3/deliverable.md`

The Developer owns only these files. Controller-owned status, contract, and
review evidence remain outside Developer ownership. Any product-file ownership
amendment requires a recorded Controller amendment and a fresh contract
Reviewer PASS before the additional path is written.

## Objective

Create the smallest pure-domain catalog boundary that can safely constrain a
later Team Draft:

1. build a bounded, immutable `TeamDraftCatalogSnapshot` from accepted
   AgentDefinition Candidates, an accepted Runtime discovery snapshot, and
   caller-supplied workspace Skill, member, permission, budget, and concurrency
   Candidates;
2. resolve AgentDefinition scope using the accepted S2-W1 precedence rules;
3. include only online RuntimeInstances and their observed model/capability
   Candidates from S2-W2;
4. compute a deterministic digest over the complete catalog and ceilings; and
5. validate proposed Team Draft references so invented or mismatched IDs fail
   closed.

This WorkItem does not create or revise a Team Draft, choose a default Main
Agent, load or instantiate a Team, bind RuntimeProfiles, allocate capacity,
write an Event, or start any process.

## Frozen catalog input

The catalog builder receives:

- zero or more S2-W1 `AgentDefinition` records;
- project and transient-generation resolution context;
- one successful S2-W2 `RuntimeDiscoverySnapshot`;
- workspace Skill IDs;
- workspace member IDs;
- allowed permission IDs;
- a non-negative budget ceiling;
- a positive concurrency ceiling; and
- positive caller-provided maximum counts for Agent definitions, online
  Runtimes, models, Skills, members, and permissions.

The maximum counts are output catalog boundaries, not authorization to expand
customer ceilings. Their counted units are frozen as follows:

- Agent maximum: selected Agent catalog entries after every input definition is
  validated and each stable ID is resolved by scope and version;
- Runtime maximum: selectable Runtime catalog entries after filtering to status
  exactly `online`;
- model maximum: Runtime-scoped `(runtime_instance_id, model_id)` pairs under
  the online Runtime entries; the same model string under two RuntimeInstance
  IDs counts as two pairs;
- Skill maximum: the normalized unique Skill ID set;
- member maximum: the normalized unique member ID set; and
- permission maximum: the normalized unique permission ID set.

Empty or duplicate workspace IDs fail validation before their normalized set is
counted. All raw AgentDefinition and Runtime observation Candidates are still
validated even when they are later context-ineligible, archived, or offline.
Bounds are evaluated after resolution, filtering, and normalization but before
the snapshot is returned. Exactly `max` entries are accepted; `max + 1` fails
closed. The builder never silently truncates, samples, or merges count units.

The zero-value Runtime discovery snapshot is invalid because it represents no
successful discovery result. A successful deterministic empty discovery
snapshot is valid and may yield zero online Runtime entries.

## Agent catalog semantics

- Every input AgentDefinition is revalidated through S2-W1 behavior.
- Stable AgentDefinition IDs are deduplicated before resolution.
- For each stable ID, the builder applies the accepted precedence:
  `matching project > reusable > matching transient generation`, then highest
  version.
- Archived or context-ineligible definitions are omitted.
- Invalid definitions and duplicate records for the same accepted S2-W1
  identity fail closed; they are not silently omitted.
- The selected catalog entry binds all catalog-relevant fields: stable ID,
  version, scope, scope identity, name, role specification, and active status.
- Entries are sorted by stable ID.

This WorkItem does not synthesize an AgentDefinition, promote a transient
definition, or reserve any definition as the default Main Agent.

## Runtime and model catalog semantics

- The builder reads Runtime observations only through the immutable S2-W2
  snapshot API.
- The S2-W2 discovery digest is included in the Team Draft catalog digest.
- Only RuntimeInstances with status exactly `online` become catalog entries.
  Offline, incompatible, and disabled observations remain valid discovery
  evidence but are excluded from selectable Team Draft references.
- Each Runtime entry binds RuntimeInstance ID, device ID, adapter type, display
  name, executable version, status, normalized capabilities, capacity,
  normalized model IDs, and source probe ID.
- Runtime entries are sorted by RuntimeInstance ID.
- Model identity is scoped to a RuntimeInstance. The same model string may
  appear under two different RuntimeInstances, but a proposed model reference
  must match the selected RuntimeInstance/model pair.

This catalog does not prove current liveness after the observation, reserve
capacity, validate a RuntimeProfile binding, confer Provider entitlement, or
authorize model invocation.

## Workspace and customer ceiling semantics

- Skill, member, and permission IDs must be non-empty and unique within their
  respective sets.
- Each set is normalized lexicographically and copied.
- Budget ceiling is an integer policy value greater than or equal to zero.
- Concurrency ceiling must be greater than zero.
- Catalog maximum counts must all be greater than zero.
- The digest includes every normalized set, every maximum count, the budget
  ceiling, and the concurrency ceiling.

The input IDs are Candidate data from later workspace/config adapters. Building
the catalog does not discover a filesystem, grant a tool, authorize a person,
or persist a customer rule.

## Proposed reference validation

The pure validator accepts a snapshot and proposed references containing:

- exactly one non-empty Main AgentDefinition ID;
- zero or more unique SubAgentDefinition IDs;
- zero or more unique RuntimeInstance IDs;
- zero or more unique Runtime/model pairs;
- zero or more unique Skill, member, and permission IDs;
- a non-negative requested budget; and
- positive requested concurrency.

Validation succeeds only when:

1. the Main AgentDefinition ID exists in the snapshot;
2. every SubAgentDefinition ID exists and differs from the Main Agent ID;
3. every RuntimeInstance ID exists in the online Runtime catalog;
4. every model is paired with a referenced RuntimeInstance and exists under
   that exact Runtime entry;
5. every Skill, member, and permission ID exists in its corresponding set;
6. requested budget is at most the snapshot budget ceiling; and
7. requested concurrency is at most the snapshot concurrency ceiling.

Empty IDs, duplicates, missing membership, Runtime/model mismatch, budget
overflow, and concurrency overflow return typed errors inspectable through
`errors.Is`. Successful validation returns a Candidate result only. It does not
accept a Team Draft, create a TeamInstance, bind an AgentInstance, or allocate
anything.

## Snapshot and digest semantics

- Snapshot state is private.
- Accessors return scalar values or fresh deep copies.
- Caller mutation of any returned collection cannot alter subsequent reads or
  the digest.
- Canonical ordering is independent of every input ordering.
- The lowercase SHA-256 digest covers the complete selected Agent entries,
  complete online Runtime/model entries, S2-W2 discovery digest, workspace and
  permission IDs, count maxima, budget ceiling, and concurrency ceiling.
- Equal canonical inputs produce equal digests under reordering; changing any
  included field class changes the digest.

The snapshot is rebuildable Candidate data, not an Event Journal fact,
persistence authority, permission grant, or second StateWriter.

## Acceptance boundary

1. `internal/teams` imports only standard-library packages plus the accepted
   `internal/agents` and `internal/runtime` domain packages.
2. No concrete CLI, Provider SDK, SQLite, Journal, projection, Evidence,
   filesystem, environment, network, process, UI, or adapter package is
   imported.
3. All inputs are validated before a successful snapshot is returned; any
   failure returns a zero snapshot.
4. Bounds fail closed without truncation.
5. Agent scope resolution and Runtime online filtering match the frozen
   semantics under input reordering.
6. Snapshot digest, ordering, and accessors are deterministic and mutation
   isolated.
7. Invented Agent, Runtime, model, Skill, member, and permission IDs are
   rejected with typed errors.
8. Runtime/model cross-pair mismatches and customer ceiling overflows are
   rejected with typed errors.
9. Tests use deterministic in-memory values only; they do not read the machine,
   PATH, environment, filesystem, credentials, network, or wall clock.
10. No TeamDefinition, TeamDraft revision/state machine, TeamInstance,
    AgentInstance, default Main Agent, question/answer/accept flow, persistence,
    migration, Event, CLI/daemon entrypoint, goroutine, RuntimeProfile binding,
    capacity allocation, Bridge, real Runtime Adapter, Run, Grant, credential
    access, or activation is introduced.
11. Existing Slice 1, S2-W1, and S2-W2 behavior remains unchanged and green.

## Mandatory RED tests

The Developer adds `internal/teams/catalog_test.go` before implementation. The
initial focused command must fail because the frozen catalog symbols do not
exist, not because of syntax, environment, dependency, or unrelated repository
failure.

Required test groups:

1. input validation for zero Runtime discovery snapshot, invalid ceilings,
   empty/duplicate workspace IDs, and `max`/`max + 1` behavior for selected
   Agents, online Runtimes, Runtime-scoped model pairs, Skills, members, and
   permissions;
2. Agent revalidation, accepted scope precedence, archived/context-ineligible
   omission, deterministic ordering, and duplicate-record failure;
3. online-only Runtime filtering, Runtime/model pairing, normalization, and
   S2-W2 discovery digest binding;
4. immutable accessors and mutation isolation for all nested collections;
5. digest reorder stability and sensitivity for every included field class;
6. successful proposed-reference validation;
7. typed rejection for empty, duplicate, invented, cross-paired, budget, and
   concurrency reference failures; and
8. static import-boundary assertion for `internal/teams/catalog.go`.

## Deterministic checks

- RED:
  `go test ./internal/teams -run 'TestTeamDraftCatalog|TestValidateTeamDraftCatalog' -count=1`
- Focused GREEN:
  `go test ./internal/teams -run 'TestTeamDraftCatalog|TestValidateTeamDraftCatalog' -count=1`
- Package full:
  `go test ./internal/teams -count=1`
- Repeated focused race:
  `go test -race ./internal/teams -run 'TestTeamDraftCatalog|TestValidateTeamDraftCatalog' -count=50`
- Impact:
  `go test ./... -count=1`
- Repository race:
  `go test -race ./... -count=1`
- Static analysis:
  `go vet ./...`
- Formatting and diff:
  `gofmt` on changed Go files and `git diff --check`
- Scope:
  verify the Candidate changes only the frozen Developer-owned files, preserves
  branch `codex/loom-platform-slice2`, and does not move HEAD before the
  authorized atomic commit.

## Required evidence

- frozen contract digest;
- exact RED terminal output and exit code;
- focused GREEN, package, repeated race, impact, repository race, and vet
  output;
- deterministic digest, bounds, membership, and mutation-isolation report;
- changed-file and import-boundary report;
- trust-boundary analysis;
- fresh independent implementation Reviewer verdict and findings;
- deliverable ending with `VERDICT: PASS` only after all gates pass.

## Trust-boundary analysis

- Catalog inputs are untrusted Candidates from domain/adapters, not authority.
- The builder filters and binds available identifiers but cannot grant access,
  reserve a Runtime, or prove continued liveness.
- A catalog digest binds the exact Candidate view used for later Draft
  validation; it is not an Event, approval, Team acceptance, or execution
  token.
- Reference validation proves only catalog membership and ceilings. It does not
  prove role suitability, RuntimeProfile compatibility, permission grant,
  customer confirmation, or execution authorization.
- Model IDs remain Runtime-scoped observed strings, not Provider entitlements.
- No raw credential, token, environment value, executable output, filesystem
  path, or Provider response enters the snapshot.

## Governance and next gate

- This freeze authorizes no product write until a fresh independent read-only
  contract Reviewer returns `PASS`.
- The active continuous Phase 1 authorization permits mandatory RED
  automatically after contract PASS.
- One Developer is the only product-file writer for this Candidate lineage.
- The Controller owns deterministic checks and state/evidence publication.
- The Developer may submit only `ready_for_review`; a fresh implementation
  Reviewer decides PASS or repair.
- Same-lineage bounded repairs follow `docs/DEVELOPMENT.md`; three failed
  product repairs require `HUMAN_REQUIRED`.
- After all checks and fresh implementation Reviewer PASS, exactly one strictly
  scoped local atomic S2-W3 commit is authorized.
- Push, merge, rebase, reset, force, release, publish, credentials, `.env`,
  dependency installation, external mutation, paid remote work, daemon or real
  Runtime activation, and autonomous execution remain prohibited.
- Team Draft revision/answer/acceptance, default Main Agent, defined-Team load,
  TeamInstance creation, Bridge/JSONL, real Runtime Adapter execution,
  AgentGrant, claim generation, WorkItem dispatch, and Run lifecycle remain
  outside this WorkItem.
