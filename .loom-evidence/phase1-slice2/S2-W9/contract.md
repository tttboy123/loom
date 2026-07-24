# S2-W9 Frozen WorkItem Contract

- ID: `S2-W9`
- Title: Explicit Agent-Mode Team Resolver
- Risk: Strict
- Status: `CONTRACT_FROZEN`
- Depends on: accepted S1-W1 mode router at `5861f82`, S2-W1 `954416a`,
  and S2-W8 `af5f158`
- Corresponds to: `PRODUCT-PLAN.md §3.2, §4.1-§4.2, §13 Phase 1`,
  `TECH-PLAN.md §3.1-§3.3, §14 Slice 2, §15.1-§15.3`, ADR-0001, and ADR-0003
- Frozen branch/head: `codex/loom-platform-slice2` at `af5f158`

## Owned files

- `internal/teams/resolver.go`
- `internal/teams/resolver_test.go`
- `.loom-evidence/phase1-slice2/S2-W9/deliverable.md`

Accepted S1 and S2-W1 through S2-W8 files remain unchanged. Any ownership
amendment requires a recorded Controller amendment and fresh contract Reviewer
PASS.

## Objective

Add a pure deterministic Team Resolver that consumes the accepted S1 Mode
Router result and returns exactly one resolution Candidate:

1. an explicitly selected saved complete Team resolves to `load_team`;
2. an explicitly selected Agent that is catalogued as a Main resolves to
   `direct_main`;
3. an explicitly selected non-Main Agent resolves to `draft_seed` containing
   the default Main plus that selected SubAgent;
4. explicit `use_agent` without a target resolves project default Team first,
   then reusable default Team, otherwise a default-Main `draft_seed`;
5. explicit `assign` resolves an exact Team or Agent target and fails on
   ambiguity; and
6. ordinary conversation, unknown triggers, malformed targets, or invalid
   catalogs fail closed with zero Candidate.

The Candidate routes a later bounded action only. It does not create a
TeamDefinition, Team Draft revision/content/decision, TeamInstance,
AgentInstance, WorkItem, process, or authority.

## Frozen input

```go
type TeamResolutionCatalogInput struct {
    AgentDefinitions       []agents.AgentDefinition
    RuntimeProfiles        []runtime.RuntimeProfile
    TeamDefinitions        []TeamDefinition
    MainAgentDefinitionIDs []string
    DefaultMainAgentID     string
    ProjectDefaultTeamID   string
    ReusableDefaultTeamID  string
}

ResolveAgentModeTeam(
    intent mode.Intent,
    context agents.ScopeIdentity,
    catalog TeamResolutionCatalogInput,
) (TeamResolutionCandidate, error)
```

Catalog rules:

1. context has a non-empty project ID and no generation ID;
2. every AgentDefinition and RuntimeProfile revalidates through accepted S2-W1
   contracts;
3. every TeamDefinition revalidates through S2-W8 against the current Agent and
   Profile catalogs;
4. Main IDs are non-empty, unique, resolvable active AgentDefinitions in the
   current context, and include the exact non-empty default Main ID;
5. optional default Team IDs are stable non-empty IDs when present;
6. the project default ID must resolve to an active project Team in the current
   project, while the reusable default ID must resolve to an active reusable
   Team;
7. all mutable inputs are copied before evaluation; and
8. catalog order does not affect resolution.

No RuntimeInstance, model-discovery result, credential, environment value,
workspace path, prompt text, or process state enters the resolution Candidate.

## Frozen result

`TeamResolutionKind` has exactly:

- `load_team`;
- `direct_main`; and
- `draft_seed`.

`TeamResolutionCandidate` contains:

- `Resolved=true`;
- derived `mode.ModeAgent`;
- exact accepted trigger and target ID, excluding `intent.Text`;
- resolution kind;
- optional copied S2-W8 `TeamDefinitionLoadCandidate` for `load_team`;
- exact Main AgentDefinition ID;
- zero or one selected SubAgentDefinition ID in S2-W9;
- `RequiresDraft=true` only for `draft_seed`; and
- a deterministic lowercase SHA-256 digest.

The digest covers derived mode, trigger, target ID, kind, TeamDefinition digest
when present, Main ID, selected SubAgent ID, and `RequiresDraft`. It never covers
or stores free-form `intent.Text`.

## Frozen resolution

Before any team lookup, the Resolver calls the accepted `mode.Route(intent)`:

- only `ModeAgent` may continue;
- caller-supplied `intent.Mode` is not authority and cannot override the
  accepted Router;
- `plain_input`, empty, unknown, or ambiguous triggers fail with
  `ErrTeamResolutionRequiresAgentMode`.

Trigger semantics:

1. `select_team`: requires non-empty target, resolves exactly one active
   S2-W8 TeamDefinition under project-over-reusable precedence, and returns
   `load_team`.
2. `select_agent`: requires non-empty target and resolves the active S2-W1
   AgentDefinition. If its stable ID is in the validated Main catalog, return
   `direct_main`; otherwise return `draft_seed` with default Main plus selected
   SubAgent.
3. `use_agent`: requires empty target. Resolve the validated project default
   Team, else reusable default Team, else return a default-Main-only
   `draft_seed`. The seed is incomplete proposal input, not S2-W5
   acceptance-ready content.
4. `assign`: requires non-empty target. Probe exact active Team and Agent
   resolution without hiding validation errors. Exactly one match follows the
   corresponding `select_team` or `select_agent` path; zero matches returns not
   found; two matches returns typed ambiguity.

Each path revalidates its selected source, returns zero Candidate on failure,
and is deterministic under catalog reorder.

Typed errors distinguish invalid resolution catalog/context, Agent-mode
required, invalid target for trigger, Agent/Team not found, ambiguous target,
invalid Main catalog/default, and propagated S2-W1/S2-W8 validation failures.

## Acceptance criteria

1. `plain_input`, unknown, empty, caller-forced Agent mode, and ordinary text
   never produce a Team resolution Candidate.
2. `select_team` returns the exact project winner before reusable and never
   creates a Draft.
3. `select_agent` returns `direct_main` for any catalogued Main and a
   default-Main-plus-selected-SubAgent `draft_seed` otherwise.
4. `use_agent` proves project default, reusable fallback, and default-Main seed
   paths in that order.
5. `assign` proves Team-only, Agent-only, not-found, and ambiguous target paths.
6. Missing/invalid default Main, duplicate/unresolvable Main IDs, wrong-scope
   default Teams, invalid Agent/Profile/Team Candidates, malformed targets, and
   invalid context fail closed.
7. Candidate does not copy `intent.Text`; changing only text leaves digest
   unchanged, while trigger/target/kind/team/Main/SubAgent/Draft semantics
   change it.
8. Catalog and source reorder do not change the Candidate or digest.
9. Input and returned-slice mutation does not change any source definition or
   subsequent resolution.
10. Every failure returns the zero Candidate.
11. Existing Slice 1 and S2-W1 through S2-W8 behavior remains green.
12. No TeamDefinition build/mutation, Team Draft revision/content/terminal
    decision, TeamInstance/AgentInstance/WorkItem/Event/SQLite write,
    persistence, allocation, workspace, process, model call, Runtime execution,
    Bridge, Run, AgentGrant, credential, daemon/CLI/UI, network, filesystem,
    environment, goroutine, or activation is introduced.

## Mandatory RED tests

The Developer adds `internal/teams/resolver_test.go` before production. RED must
fail on missing frozen S2-W9 symbols only.

Required groups:

1. Router gate including caller-forced Mode Agent and text exclusion;
2. selected Team project/reusable precedence and no-Draft result;
3. selected catalogued Main and selected non-Main paths;
4. use-Agent project default, reusable fallback, and default-Main seed;
5. assign Team-only, Agent-only, zero, and ambiguous matches;
6. all catalog/context/Main/default/target/source failures with zero output;
7. deterministic reorder, digest field sensitivity, and mutation isolation; and
8. static import/trust-boundary assertion.

## Deterministic checks

- RED/focused GREEN:
  `go test ./internal/teams -run 'TestResolveAgentModeTeam' -count=1`
- Package full:
  `go test ./internal/teams -count=1`
- Repeated focused race:
  `go test -race ./internal/teams -run 'TestResolveAgentModeTeam' -count=50`
- Impact:
  `go test ./... -count=1`
- Repository race:
  `go test -race ./... -count=1`
- Static:
  `go vet ./...`
- Formatting/diff:
  `gofmt -d internal/teams/resolver.go internal/teams/resolver_test.go` and
  `git diff --check`
- Import boundary: standard library plus accepted `internal/agents`,
  `internal/mode`, and `internal/runtime` contracts only.

## Explicit exclusions

No changes to accepted S1 or S2-W1 through S2-W8 product/test files. No saved
TeamDefinition build/mutation, Draft creation, default Main AgentDefinition
creation, TeamInstance/AgentInstance/WorkItem creation, RuntimeInstance binding,
transaction, Event/SQLite write, persistence, process/model/runtime execution,
Bridge, Run, AgentGrant, credential, network/filesystem/environment access,
goroutine, daemon/CLI/UI, external action, or Slice 3 behavior.

VERDICT: CONTRACT_FROZEN
