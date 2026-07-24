# S2-W1 Deliverable: Agent and Runtime Catalog Domain Contracts

## 1. Changed what

- Added `internal/agents/definition.go` with validated, versioned
  `AgentDefinition`, explicit project/reusable/transient scope identity, typed
  errors, and deterministic scope/version resolution.
- Added `internal/runtime/catalog.go` with separate validated
  `RuntimeProfile` and observed `RuntimeInstance` values, immutable capability
  copies, typed binding failures, and side-effect-free `BindingCandidate`
  validation.
- Added the mandatory RED tests before production implementation in
  `internal/agents/definition_test.go` and
  `internal/runtime/catalog_test.go`.
- Product Candidate status is `READY_FOR_REVIEW`. The Developer did not mark
  the WorkItem done or accepted.

## 2. Success and exact evidence

### Frozen contract

```text
contract_sha256=3b3a54f71419d59ac67c81edface6bcc1dd2ff9c6603bc4c3cce83be4bc599d0
contract_review=PASS
contract_review_findings=none blocking
```

### Mandatory RED

The two test files were created before either production file. The frozen RED
command exited `1` because the required domain symbols did not exist:

```text
# loom-pi-rebuild/internal/runtime [loom-pi-rebuild/internal/runtime.test]
internal/runtime/catalog_test.go:18:18: undefined: RuntimeProfile
internal/runtime/catalog_test.go:23:25: undefined: AuthBrokered
internal/runtime/catalog_test.go:28:18: undefined: NewRuntimeProfile
internal/runtime/catalog_test.go:41:19: undefined: RuntimeInstance
# loom-pi-rebuild/internal/agents [loom-pi-rebuild/internal/agents.test]
internal/agents/definition_test.go:16:11: undefined: AgentDefinition
internal/agents/definition_test.go:19:12: undefined: ScopeProject
internal/agents/definition_test.go:20:18: undefined: ScopeIdentity
internal/agents/definition_test.go:25:13: undefined: DefinitionActive
internal/agents/definition_test.go:28:21: undefined: NewAgentDefinition
FAIL
red_exit=1
red_failure_reason=missing_frozen_domain_symbols
red_environment_or_syntax_failure=NONE
```

The test digests were unchanged after implementation:

```text
df7d5511d4f914690e7bd1c686305cc2ecd207322d6f0f5153340fe61557fa78  internal/agents/definition_test.go
b74220993f4e41755c6229e21a22cf0f6dcd7f022344aa4bbc682f7349357ec8  internal/runtime/catalog_test.go
```

### Focused GREEN and package checks

```text
ok  loom-pi-rebuild/internal/agents
ok  loom-pi-rebuild/internal/runtime
focused_green_exit=0
ok  loom-pi-rebuild/internal/agents
ok  loom-pi-rebuild/internal/runtime
package_full_exit=0
```

### Strict race checks

```text
ok  loom-pi-rebuild/internal/agents
ok  loom-pi-rebuild/internal/runtime
focused_race_50_exit=0
```

### Impact and repository race

```text
ok  loom-pi-rebuild/cmd/loom
ok  loom-pi-rebuild/internal/agents
ok  loom-pi-rebuild/internal/evidence
ok  loom-pi-rebuild/internal/journal
ok  loom-pi-rebuild/internal/mode
ok  loom-pi-rebuild/internal/projection
ok  loom-pi-rebuild/internal/runtime
?   loom-pi-rebuild/migrations [no test files]
impact_exit=0

ok  loom-pi-rebuild/cmd/loom
ok  loom-pi-rebuild/internal/agents
ok  loom-pi-rebuild/internal/evidence
ok  loom-pi-rebuild/internal/journal
ok  loom-pi-rebuild/internal/mode
ok  loom-pi-rebuild/internal/projection
ok  loom-pi-rebuild/internal/runtime
?   loom-pi-rebuild/migrations [no test files]
repository_race_exit=0
vet_exit=0
```

### Format, scope, and trust boundary

```text
gofmt_check=PASS
git_diff_check=PASS
branch_head_unchanged=PASS
frozen_existing_product_diff=NONE
owned_product_scope=PASS
agents_imports=errors,fmt
runtime_imports=errors,fmt,sort,time
```

Production file digests:

```text
d82e67105138620b4774721adb5e5c660016e3c3be6498b5aa932dc0a93a786a  internal/agents/definition.go
a892e6bda42f0970f384084f9dab97a93cfa0b82ee7596cc42a8cdc94789d3a9  internal/runtime/catalog.go
```

Fresh implementation Review 1 returned `FAIL` after all deterministic checks
passed. The preserved findings are in `review-1.md`; Repair 1 is frozen in
`repair-1-contract.md`.

### Repair 1 RED and GREEN

Repair 1 added the mixed-stable-ID and empty requested-ID regression before
changing production code. The RED command exited `1` on the exact fail-open
behavior:

```text
definition_test.go:206: ResolveDefinition(empty definition id) error = <nil>, want ErrAgentDefinitionNotFound
repair1_red_exit=1
repair1_red_failure_reason=empty_definition_id_fail_open
```

The minimal repair requires an explicit Definition ID, removes empty-ID
wildcard matching, and resolves versions only within that stable ID. The
profile-switching proof now uses two external selection values with distinct
profile IDs and the same validated AgentDefinition; reflection still proves
AgentDefinition contains no Runtime/Profile/Provider/model authority field.

Fresh post-repair evidence:

```text
repair1_focused_green_exit=0
repair1_package_full_exit=0
repair1_focused_race_50_exit=0
repair1_impact_exit=0
repair1_repository_race_exit=0
repair1_vet_exit=0
repair1_gofmt_check=PASS
repair1_git_diff_check=PASS
repair1_branch_head_unchanged=PASS
repair1_frozen_existing_product_diff=NONE
repair1_owned_product_scope=PASS
agents_imports=errors,fmt
runtime_imports=errors,fmt,sort,time
```

Repair 1 Candidate digests:

```text
97a1918e06b088f3a27ef10ea3ff198d15ddae2e84d3465feb1b6a0f6b699788  internal/agents/definition.go
1881b873b8e74898e9cf930215ab519a86363ced263c5f50072f25aa33d92d82  internal/agents/definition_test.go
a892e6bda42f0970f384084f9dab97a93cfa0b82ee7596cc42a8cdc94789d3a9  internal/runtime/catalog.go
b74220993f4e41755c6229e21a22cf0f6dcd7f022344aa4bbc682f7349357ec8  internal/runtime/catalog_test.go
```

Fresh Repair 1 implementation Review 2 returned `PASS` with no blocking
findings. Its immutable evidence is preserved in `review-2.md`.

## 3. Impacted files and product behavior

Owned product files:

- `internal/agents/definition.go`
- `internal/agents/definition_test.go`
- `internal/runtime/catalog.go`
- `internal/runtime/catalog_test.go`

The new behavior is pure and local: validate Agent and Runtime catalog values,
resolve a definition deterministically, and validate a proposed compatible
online Runtime binding. It does not persist, discover, allocate, dispatch,
authenticate, invoke a Runtime, start a process, create Team/Agent/Run state,
write an Event, issue a Grant, or activate anything.

No Go dependency, migration, CLI, Journal, Evidence, projection, mode, daemon,
Bridge, credential, external resource, or existing Slice 1 product file
changed.

## 4. Residual and unverified risk

- Review 1 `FAIL` remains preserved. Repair 1 passed all Controller checks and
  fresh independent Review 2 with no blocking findings.
- This WorkItem does not prove Runtime discovery, catalog snapshot membership,
  invented-ID rejection, Team Draft behavior, persistence, daemon behavior, or
  any live Runtime path; those remain later frozen WorkItems.
- Nothing is committed, pushed, merged, released, or activated.

## 5. Next executable step

Create the authorized strictly scoped local S2-W1 atomic commit, excluding
pre-existing `AGENTS.md`, `.codex/`, `.loom-drafts/`, and unrelated progress
drafts. Then freeze the next bounded Slice 2 WorkItem contract.

## 6. Current verdict

VERDICT: PASS
