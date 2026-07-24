# S2-W1 Frozen WorkItem Contract

- ID: `S2-W1`
- Title: Agent and Runtime Catalog Domain Contracts
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: Slice 1 PASS at commit `5861f82`
- Corresponds to: `TECH-PLAN.md §4, §4.1, §13.1, §14 Slice 2.1 and
  Slice 2.3, §15.7-§15.8`
- Frozen branch/head: `codex/loom-platform-slice2` at `f0820be`

## Owned files

- `internal/agents/definition.go`
- `internal/agents/definition_test.go`
- `internal/runtime/catalog.go`
- `internal/runtime/catalog_test.go`
- `.loom-evidence/phase1-slice2/S2-W1/deliverable.md`

The Developer owns only these files. Any ownership amendment requires a new
Controller decision before writing the additional path.

## Objective

Create the smallest pure-domain foundation for the first Slice 2 boundary:

1. model reusable, versioned `AgentDefinition` records without embedding model,
   Provider, device, executable, or live Runtime state;
2. model `RuntimeProfile` separately from discovered `RuntimeInstance`;
3. resolve AgentDefinition scope deterministically; and
4. validate a proposed RuntimeProfile-to-RuntimeInstance binding without
   starting a Run or creating an AgentInstance.

These types and validators are domain contracts only. They do not create a
catalog authority, persistence layer, discovery process, daemon, Team, Draft,
Agent process, or execution permission.

## Frozen domain boundary

### AgentDefinition

An AgentDefinition has:

- non-empty stable ID;
- positive version;
- scope `project`, `reusable`, or `transient`;
- a scope identity containing a project ID only for `project`, a generation ID
  only for `transient`, and neither for `reusable`;
- non-empty name;
- non-empty role specification;
- status `active` or `archived`.

The definition contains no RuntimeProfile ID, adapter, Provider, model,
authentication mode, executable/device state, capacity, credential, secret, or
Run state.

### Scope resolution

For definitions with the same stable ID, resolution precedence is:

```text
matching project > reusable > matching transient generation
```

- A project definition is eligible only for its exact project.
- A transient definition is eligible only for its exact non-empty generation.
- A transient definition never becomes reusable or project-scoped.
- Within the winning scope, the highest positive version wins.
- Duplicate records for the same `(id, version, scope identity)` fail closed.
- Archived definitions are never selected.
- No eligible definition returns a typed not-found error rather than an empty
  success.

### RuntimeProfile

A RuntimeProfile has:

- non-empty stable ID;
- non-empty adapter type;
- optional Provider and model identifiers;
- authentication mode `brokered`, `provider_ephemeral`, or `native_auth`;
- a deterministic set of required capabilities;
- positive timeout;
- an optional non-negative budget.

It contains no RuntimeInstance ID, device ID, executable version, online state,
capacity observation, raw Provider key, OAuth refresh token, CLI credential, or
Loom grant token.

### RuntimeInstance

A RuntimeInstance has:

- non-empty stable ID and device ID;
- non-empty adapter type and display name;
- optional executable version;
- status `online`, `offline`, `incompatible`, or `disabled`;
- a deterministic set of observed capabilities;
- positive capacity.

It contains no AgentDefinition role specification, Provider credential, model
policy, budget policy, timeout policy, or Run state.

### Binding validation

A proposed binding is valid only when:

1. both records independently satisfy their frozen validation rules;
2. the RuntimeInstance status is exactly `online`;
3. adapter types match exactly;
4. every required RuntimeProfile capability is present in the observed
   RuntimeInstance capability set; and
5. RuntimeInstance capacity is positive.

Offline, incompatible, disabled, adapter-mismatched, capability-missing, and
invalid-capacity candidates fail with typed errors. Validation returns a
Candidate result only; it does not allocate capacity, create AgentInstance or
Run state, issue a Grant, invoke a Runtime, or write an Event.

## Acceptance boundary

1. `internal/agents` and `internal/runtime` remain pure domain packages and do
   not import CLI, SQLite, Journal, projection, Evidence, Provider SDK, concrete
   Runtime adapter, network, process, or UI packages.
2. Constructors or validators reject every invalid enum, empty required ID,
   invalid version/capacity, illegal scope identity, and non-positive timeout.
3. AgentDefinition scope resolution is deterministic under input reordering and
   implements the frozen precedence and highest-version rules.
4. Runtime binding validation accepts compatible online instances and rejects
   every frozen failure class with errors that callers can inspect using
   `errors.Is`.
5. Returned definitions, profiles, instances, and capability collections do
   not expose mutable aliases to caller-owned input.
6. Tests prove AgentDefinition is independent of RuntimeProfile and that
   RuntimeProfile switching does not require cloning or mutating the
   AgentDefinition.
7. Tests prove offline, incompatible, disabled, adapter-mismatched,
   capability-missing, and zero-capacity RuntimeInstance candidates cannot
   produce an accepted binding. Catalog-snapshot membership and invented-ID
   rejection remain a later Team Draft catalog WorkItem under `§15.9`.
8. No persistence schema or migration is introduced. Slice 2 catalog
   persistence and Runtime discovery remain separately frozen future
   WorkItems.
9. No TeamDefinition, TeamDraft, TeamInstance, AgentInstance, default Main
   Agent, directory snapshot, daemon, Bridge, CLI command, network call,
   background process, Runtime invocation, Run, Grant, credential access, or
   product activation is introduced.
10. Existing Slice 1 behavior and tests remain unchanged and green.

## Mandatory RED tests

The Developer adds tests before implementation. The initial focused command must
fail because the new domain contracts and validators do not exist, not because
of syntax, environment, dependency, or unrelated repository failure.

Required test groups:

1. AgentDefinition validation and immutable separation from RuntimeProfile.
2. Project/reusable/transient resolution precedence, version selection,
   reorder determinism, duplicate rejection, archived exclusion, and typed
   not-found behavior.
3. RuntimeProfile and RuntimeInstance validation.
4. Compatible online binding success.
5. Typed rejection for offline, incompatible, disabled, adapter mismatch,
   missing capabilities, and invalid capacity.
6. Mutation-isolation checks for all caller-provided slices/maps.
7. Static import-boundary assertion for the two new domain packages.

## Deterministic checks

- RED:
  `go test ./internal/agents ./internal/runtime -run 'Test(AgentDefinition|ResolveDefinition|RuntimeProfile|RuntimeInstance|ValidateBinding)' -count=1`
- Focused GREEN:
  `go test ./internal/agents ./internal/runtime -run 'Test(AgentDefinition|ResolveDefinition|RuntimeProfile|RuntimeInstance|ValidateBinding)' -count=1`
- Repeated focused race:
  `go test -race ./internal/agents ./internal/runtime -count=50`
- Package full:
  `go test ./internal/agents ./internal/runtime -count=1`
- Impact:
  `go test ./... -count=1`
- Repository race:
  `go test -race ./... -count=1`
- Static analysis:
  `go vet ./...`
- Formatting and diff:
  `gofmt` on changed Go files, trailing-whitespace check, and
  `git diff --check`
- Scope:
  verify the Candidate changes only the frozen owned files and preserves branch
  `codex/loom-platform-slice2` without moving HEAD.

## Required Evidence

- frozen contract digest;
- exact RED terminal output and exit code;
- focused GREEN, repeated race, package, impact, repository race, and vet
  terminal output;
- changed-file and import-boundary report;
- trust-boundary analysis;
- fresh independent Reviewer verdict and findings;
- deliverable ending with `VERDICT: PASS` only after all gates pass.

## Trust-boundary analysis

- AgentDefinition is role authority, not Runtime or credential authority.
- RuntimeProfile is execution policy Candidate material, not a discovered
  Runtime or permission to run.
- RuntimeInstance is observed local capability state, not a role, policy, Grant,
  or execution decision.
- Binding validation is fail-closed and side-effect free. It cannot allocate,
  persist, dispatch, authenticate, or start a process.
- Secrets and raw authentication material are outside all three domain
  contracts.
- Model, Agent, Runtime, and catalog outputs remain Proposal or Candidate
  material until later governed acceptance.

## Governance and next gate

- Contract freezing does not authorize RED, implementation, dependency changes,
  migration changes, or product behavior.
- One Developer writer may start only after explicit human authorization for
  this frozen contract.
- The Controller owns deterministic test execution.
- A fresh read-only Reviewer must return PASS before S2-W2 can be frozen.
- Three failed bounded product repairs in this lineage require
  `HUMAN_REQUIRED`.
- No commit, push, merge, release, daemon or Runtime activation, credential
  change, dependency installation, paid remote work, external mutation, or
  FastContext installation is authorized.
- Bridge/JSON-RPC/JSONL, AgentGrant, claim generation, prepare lease, WorkItem
  dispatch, and real Runtime Adapter behavior remain Slice 3 boundaries under
  `TECH-PLAN.md §14`; research drafts cannot move them into this WorkItem.
